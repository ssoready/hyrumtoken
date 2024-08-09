# hyrumtoken

[![Go Reference](https://pkg.go.dev/badge/github.com/ssoready/hyrumtoken.svg)](https://pkg.go.dev/github.com/ssoready/hyrumtoken)

`hyrumtoken` is a Go package to encrypt pagination tokens, so that your API
clients can't depend on their contents, ordering, or any other characteristics.

## Installation

```bash
go get github.com/ssoready/hyrumtoken
```

## Usage

`hyrumtoken.Marshal/Unmarshal` works like the equivalent `json` functions,
except they take a `key *[32]byte`:

```go
var key [32]byte = ...

// create an encrypted pagination token
token, err := hyrumtoken.Marshal(&key, "any-json-encodable-data")

// parse an encrypted pagination token
var parsedToken string
err := hyrumtoken.Unmarshal(&key, token, &parsedToken)
```

You can use any data type that works with `json.Marshal` as your pagination
token.

## Motivation

[Hyrum's Law](https://www.hyrumslaw.com/) goes:

> With a sufficient number of users of an API, it does not matter what you promise in the contract: all observable
behaviors of your system will be depended on by somebody.

Pagination tokens are one of the most common ways this turns up. I'll illustrate
with a story.

### Getting stuck with LIMIT/OFFSET

I was implementing an audit logging feature. My job was the backend, some other
folks were doing the frontend. To get them going quickly, I gave them an API
documented like this:

> To list audit log events, do `GET /v1/events?pageToken=...`. For the first
page, use an empty `pageToken`.
>
> That will return `{"events": [...], "nextPageToken": "...", "totalCount": ...}`.
If `nextPageToken` is empty, you've hit the end of the list. 

To keep things real simple, my unblock-the-frontend MVP used `limit/offset`
pagination. The page tokens were just the `offset` values. This wasn't going to
work once we had filters/sorts/millions of events, but whatever! Just rendering
the audit log events was already a good chunk of work for the frontend folks,
and we wanted to work in parallel.

A week ensues. The frontend folks came back with a UI that had one of these at
the bottom:

![](./screenshot.png)

Weird. The documented API doesn't really promise any affordance of "seeking" to
a random page. "If you're on page 1 and you click on 3, what happens?" The
reply: "We just set the pageToken to 300".

This happened because folks saw the initial real-world behavior of the API:

```
GET /v1/events
{"events": [... 100 events ...], "nextPageToken": "100", "totalCount": "8927"}

GET /v1/events?pageToken=100
{"events": [... 100 events ...], "nextPageToken": "200", "totalCount": "8927"}
```

And so it didn't matter what you document. People will guess what you meant, and
it really looks like you meant to make `pageToken` be an offset token.

The fun part about this story is that I in fact have lied to you. We *knew*
keyset-based pagination was coming, and so we needed a way to encode potentially
URL-unsafe data in `pageToken`. So right from the get-go we were base64-encoding
the token. So the actual requests looked like:

```
GET /v1/events
{"events": [... 100 events ...], "nextPageToken": "MTAwCg==", "totalCount": "8927"}

GET /v1/events?pageToken=MTAwCg==
{"events": [... 100 events ...], "nextPageToken": "MjAwCg==", "totalCount": "8927"}
```

The effect is the same. If it ends in `==`, you bet your ass the intellectual
curiosity of your coworkers demands they base64-parse it. Parse `MTAwCg==` and
you get back `100\n`. Our company design system had a prebuilt component with a
jump-to-page affordance, and the UX folks put two and two together
instinctively.

By making an API that looked like it wanted to let you "seek" through the data,
I had invited my colleagues to design and implement a user interface that I had
no plans to support. This problem was on me.

In a lot of ways, I got lucky here. I can just politely ask my coworkers to
redesign their frontend to only offer a "Load More" button, no "jump to page".
If I had made this API public, paying customers would have read the tea-leaves
of my API, and they'd be broken if I changed anything. We'd probably be stuck
with the limit/offset approach forever.

### Binary searching through pagination-token-space

I've been on the opposite end of this. In the past, I've worked at companies
that had to ETL data out of systems faster than the public API would allow. Each
individual request is slow, but parallel requests increased throughput out of
their API. Problem was figuring out how to usefully do parallel requests over a
paginated list.

We figured out that their pagination tokens were alphabetically increasing, and
so we made a program that "searched" for the last pagination token, divided up
the pagination token space into *N* chunks, and synced those chunks in parallel.

Probably not what they intended! But in practice we're now one of the biggest
users of their API, and they can't change their behavior. Even the *alphabetical
ordering* of your pagination tokens can get you stuck.

At that same company, we would sometimes parse pagination tokens to implement
internal logging of where we were in the list. This might seem gratuitous, but
engineers are always tempted to do this. 

If you didn't want me to parse your sorta-opaque token, you should've made it
actually-opaque.

### Encrypt your pagination tokens

So that's why I like to encrypt my pagination tokens. It seems extreme, but it
eliminates this entire class of problems. Instead of obscurity-by-base64, I just
enforce opacity-by-Salsa20.

`hyrumtoken` prevents your users from:

1. Creating their own pagination tokens to "seek" through your data
2. Parsing your returned pagination tokens to infer where they are in the data
3. Having their software be broken if you change what you put inside your
   pagination tokens

If you intend your pagination tokens to be opaque strings, `hyrumtoken` can
enforce that opacity. Concretely, `hyrumtoken` does this:

1. JSON-encode the "pagination state" data
2. Encrypt that using NaCL's [secretbox](https://nacl.cr.yp.to/secretbox.html)
   with a random nonce. This requires a secret key, hence the need for a `key
   *[32]byte`.
3. Concatenate the nonce and the encrypted message
4. Return a base64url-encoded copy

Secretbox is implemented using Golang's widely-used [`x/crypto/nacl/secretbox`
package](https://pkg.go.dev/golang.org/x/crypto/nacl/secretbox). There are
Secretbox implementations in every language, so it's pretty easy to port or
share tokens between backend languages.

## Advanced Usage

### Expiring tokens

This one isn't particularly tied to `hyrumtoken`.

Your customers may get into the habit of assuming your pagination tokens never
expire (again in the spirit of Hyrum's Law). You can enforce that by having
tokens keep track of their own expiration:

```go
type tokenData struct {
	ExpireTime time.Time
	ID         string
}

// encode
hyrumtoken.Marshal(&key, tokenData{
	ExpireTime: time.Now().Add(time.Hour),
	ID: ...,
})

// decode
var data tokenData
if err := hyrumtoken.Unmarshal(&key, token, &data); err != nil {
	return err
}
if data.ExpireTime.Before(time.Now()) {
	return fmt.Errorf("token is expired")
}
```

That way, your customer probably sees they're wrong to assume "tokens never
expire" while they're still developing their software, and that assumption is
still easy to undo.

### Rotating keys

Any time you have keys, you should think about how you're gonna rotate them. It
might be obvious, but you can just have a "primary" key you encode new tokens
with, and a set of "backup" keys you try to decode with. Something like this:

```go
var primaryKey [32]byte = ...
var backupKey1 [32]byte = ...
var backupKey2 [32]byte = ...

// encode
token, err := hyrumtoken.Marshal(&key, data)

// decode
keys := [][32]byte{primaryKey, backupKey1, backupKey2}
for _, k := range keys {
	var data tokenData 
	if err := hyrumtoken.Unmarshal(&k, token, &data); err == nil {
		return &data, nil
	}
}
return nil, fmt.Errorf("invalid pagination token")
```

You can use expiring tokens to eventually guarantee the backup keys are never
used, and stop accepting them entirely.

### Changing pagination schemes

You can change from one type of pagination to another by putting both into the
same struct, and then looking at which fields are populated:

```go
type tokenData struct {
	Offset  int
	StartID string
}

var data tokenData
if err := hyrumtoken.Unmarshal(&key, token, &data); err != nil {
	return err
}

if data.Offset != 0 {
	// offset-based approach
}
// startid-based approach
```

Expiring tokens also help here, so you can get rid of the old codepath quickly.

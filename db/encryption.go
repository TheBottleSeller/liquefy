package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

//32-bit AES encryption key
//WARNING: changing this means being unable to decrypt secrets encrypted with another key
var LQ_CIPHER_KEY = []byte("bydaf8n3h1pf1cmajtfajs39gd3ic2tr")

type lqEncoder interface {
	Encode(src []byte) []byte
	Decode(src []byte) ([]byte, error)
}

type Base64Encoder struct{}

func (b *Base64Encoder) Encode(src []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(src))
}

func (b *Base64Encoder) Decode(src []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(src))
}

type AESEncoder struct{}

// Encode encodes plaintext into AES ciphertext and finally hex encodes the ciphertext to store in DB
// Mostly ripped from https://golang.org/pkg/crypto/cipher/#example_NewCFBEncrypter
func (a *AESEncoder) Encode(src []byte) []byte {
	block, err := aes.NewCipher(LQ_CIPHER_KEY)
	if err != nil {
		panic(err)
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	ciphertext := make([]byte, aes.BlockSize+len(src))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], src)

	// It's important to remember that ciphertexts must be authenticated
	// (i.e. by using crypto/hmac) as well as being encrypted in order to
	// be secure.

	// Hex encode ciphertext so we can store in the DB
	dst := make([]byte, hex.EncodedLen(len(ciphertext)))
	hex.Encode(dst, ciphertext)
	return dst
}

// Decode decodes a hex-encoded bytestring to AES ciphertext and then to plaintext
// Mostly ripped from https://golang.org/pkg/crypto/cipher/#example_NewCFBDecrypter
func (a *AESEncoder) Decode(src []byte) ([]byte, error) {
	// First things first decode hex text to ciphertext
	ciphertext := make([]byte, hex.DecodedLen(len(src)))
	hex.Decode(ciphertext, src)

	block, err := aes.NewCipher(LQ_CIPHER_KEY)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)
	return ciphertext, nil
}

func NewEncoder() lqEncoder {
	// The Base64Encoder is far more naive and won't be as secure
	// so default to AES
	return &AESEncoder{}
}

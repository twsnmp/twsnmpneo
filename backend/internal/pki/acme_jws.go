package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

type acmeJWS struct {
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type acmeProtected struct {
	Alg   string          `json:"alg"`
	Nonce string          `json:"nonce"`
	URL   string          `json:"url"`
	KID   string          `json:"kid,omitempty"`
	JWK   json.RawMessage `json:"jwk,omitempty"`
	Crit  []string        `json:"crit,omitempty"`
	B64   *bool           `json:"b64,omitempty"`
}

type acmeJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
}

type parsedACMEJWS struct {
	protected  acmeProtected
	payload    []byte
	publicKey  crypto.PublicKey
	thumbprint string
	input      []byte
	signature  []byte
}

func decodeACMEJWS(raw []byte, expectedURL string) (parsedACMEJWS, error) {
	var jws acmeJWS
	if err := json.Unmarshal(raw, &jws); err != nil {
		return parsedACMEJWS{}, fmt.Errorf("malformed JWS JSON")
	}
	protectedBytes, err := base64.RawURLEncoding.DecodeString(jws.Protected)
	if err != nil || len(protectedBytes) > 4096 {
		return parsedACMEJWS{}, fmt.Errorf("invalid JWS protected header")
	}
	var protected acmeProtected
	if err := json.Unmarshal(protectedBytes, &protected); err != nil {
		return parsedACMEJWS{}, fmt.Errorf("invalid JWS protected header")
	}
	var protectedMembers map[string]json.RawMessage
	if err := json.Unmarshal(protectedBytes, &protectedMembers); err != nil {
		return parsedACMEJWS{}, fmt.Errorf("invalid JWS protected header")
	}
	for name := range protectedMembers {
		if name != "alg" && name != "nonce" && name != "url" && name != "kid" &&
			name != "jwk" && name != "crit" && name != "b64" {
			return parsedACMEJWS{}, fmt.Errorf("unsupported protected JWS header member")
		}
	}
	if protected.URL != expectedURL {
		return parsedACMEJWS{}, fmt.Errorf("JWS url does not match request URL")
	}
	if protected.Nonce == "" {
		return parsedACMEJWS{}, fmt.Errorf("JWS nonce is required")
	}
	if len(protected.Crit) != 0 || protected.B64 != nil {
		return parsedACMEJWS{}, fmt.Errorf("unsupported critical JWS header")
	}
	if (protected.KID == "") == (len(protected.JWK) == 0) {
		return parsedACMEJWS{}, fmt.Errorf("JWS must contain exactly one of kid or jwk")
	}
	payload, err := base64.RawURLEncoding.DecodeString(jws.Payload)
	if err != nil {
		return parsedACMEJWS{}, fmt.Errorf("invalid JWS payload encoding")
	}
	sig, err := base64.RawURLEncoding.DecodeString(jws.Signature)
	if err != nil {
		return parsedACMEJWS{}, fmt.Errorf("invalid JWS signature encoding")
	}
	input := []byte(jws.Protected + "." + jws.Payload)
	var pub crypto.PublicKey
	var thumbprint string
	if len(protected.JWK) != 0 {
		var members map[string]json.RawMessage
		if err := json.Unmarshal(protected.JWK, &members); err != nil {
			return parsedACMEJWS{}, fmt.Errorf("invalid JWK")
		}
		for name := range members {
			if name != "kty" && name != "crv" && name != "x" && name != "y" && name != "n" && name != "e" {
				return parsedACMEJWS{}, fmt.Errorf("private or unsupported JWK members are not accepted")
			}
		}
		var jwk acmeJWK
		if err := json.Unmarshal(protected.JWK, &jwk); err != nil {
			return parsedACMEJWS{}, fmt.Errorf("invalid JWK")
		}
		pub, thumbprint, err = parseACMEJWK(jwk)
		if err != nil {
			return parsedACMEJWS{}, err
		}
		if !validACMEAlgorithm(protected.Alg, pub) {
			return parsedACMEJWS{}, fmt.Errorf("unsupported JWS algorithm for JWK")
		}
		if err := verifyACMESignature(protected.Alg, pub, input, sig); err != nil {
			return parsedACMEJWS{}, err
		}
	}
	return parsedACMEJWS{
		protected: protected, payload: payload, publicKey: pub, thumbprint: thumbprint,
		input: input, signature: sig,
	}, nil
}

func parseACMEJWK(jwk acmeJWK) (crypto.PublicKey, string, error) {
	var canonical string
	switch jwk.Kty {
	case "EC":
		var curve elliptic.Curve
		switch jwk.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		default:
			return nil, "", fmt.Errorf("unsupported JWK curve")
		}
		x, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			return nil, "", fmt.Errorf("invalid JWK x coordinate")
		}
		y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
		if err != nil {
			return nil, "", fmt.Errorf("invalid JWK y coordinate")
		}
		pub := &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if !curve.IsOnCurve(pub.X, pub.Y) || len(x) != (curve.Params().BitSize+7)/8 || len(y) != (curve.Params().BitSize+7)/8 {
			return nil, "", fmt.Errorf("invalid JWK EC public key")
		}
		canonical = `{"crv":"` + jwk.Crv + `","kty":"EC","x":"` + jwk.X + `","y":"` + jwk.Y + `"}`
		return pub, jwkThumbprint(canonical), nil
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			return nil, "", fmt.Errorf("invalid JWK modulus")
		}
		eb, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil || len(nb) < 256 || len(eb) == 0 || len(eb) > 4 {
			return nil, "", fmt.Errorf("invalid or undersized JWK RSA key")
		}
		e := 0
		for _, b := range eb {
			e = e<<8 | int(b)
		}
		if e < 3 || e%2 == 0 {
			return nil, "", fmt.Errorf("invalid JWK RSA exponent")
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
		if pub.N.BitLen() < 2048 {
			return nil, "", fmt.Errorf("JWK RSA key must be at least 2048 bits")
		}
		canonical = `{"e":"` + jwk.E + `","kty":"RSA","n":"` + jwk.N + `"}`
		return pub, jwkThumbprint(canonical), nil
	default:
		return nil, "", fmt.Errorf("unsupported JWK key type")
	}
}

func jwkThumbprint(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func verifyACMESignature(alg string, pub crypto.PublicKey, input, signature []byte) error {
	var digest []byte
	var hash crypto.Hash
	switch alg {
	case "ES256", "RS256":
		sum := sha256.Sum256(input)
		digest, hash = sum[:], crypto.SHA256
	case "ES384":
		sum := sha512.Sum384(input)
		digest, hash = sum[:], crypto.SHA384
	default:
		return fmt.Errorf("unsupported JWS algorithm")
	}
	switch key := pub.(type) {
	case *ecdsa.PublicKey:
		if alg != "ES256" && alg != "ES384" {
			return fmt.Errorf("JWS algorithm does not match JWK")
		}
		size := (key.Curve.Params().BitSize + 7) / 8
		if len(signature) != 2*size {
			return fmt.Errorf("invalid ECDSA JWS signature size")
		}
		r := new(big.Int).SetBytes(signature[:size])
		s := new(big.Int).SetBytes(signature[size:])
		if !ecdsa.Verify(key, digest, r, s) {
			return fmt.Errorf("JWS signature verification failed")
		}
	case *rsa.PublicKey:
		if alg != "RS256" {
			return fmt.Errorf("JWS algorithm does not match JWK")
		}
		if err := rsa.VerifyPKCS1v15(key, hash, digest, signature); err != nil {
			return fmt.Errorf("JWS signature verification failed")
		}
	default:
		if pub != nil {
			return fmt.Errorf("unsupported JWK public key")
		}
		return nil
	}
	return nil
}

func validACMEAlgorithm(alg string, pub crypto.PublicKey) bool {
	switch key := pub.(type) {
	case *ecdsa.PublicKey:
		return (alg == "ES256" && key.Curve == elliptic.P256()) ||
			(alg == "ES384" && key.Curve == elliptic.P384())
	case *rsa.PublicKey:
		return alg == "RS256"
	default:
		return false
	}
}

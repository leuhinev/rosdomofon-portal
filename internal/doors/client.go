// internal/doors/client.go
package doors

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Door struct {
	URL         string
	Method      string
	AuthType    string
	Username    string
	Password    string
	Body        string
	ContentType string
	InsecureTLS bool
}

// ProxyRequest выполняет запрос к конечному устройству с указанным типом авторизации
func ProxyRequest(door Door) (*http.Response, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: door.InsecureTLS,
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_RSA_WITH_RC4_128_SHA,
					tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
					tls.TLS_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
			},
		},
	}

	var req *http.Request
	var err error

	bodyReader := bytes.NewBufferString("")
	if door.Body != "" {
		bodyReader = bytes.NewBufferString(door.Body)
	}

	req, err = http.NewRequest(door.Method, door.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "curl/7.81.0")
	if door.ContentType != "" {
		req.Header.Set("Content-Type", door.ContentType)
	} else if door.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	switch door.AuthType {
	case "basic":
		req.SetBasicAuth(door.Username, door.Password)
		return client.Do(req)
	case "digest":
		return doDigestRequest(client, req, door.Username, door.Password)
	case "none":
		return client.Do(req)
	default:
		return nil, fmt.Errorf("unknown auth_type: %s", door.AuthType)
	}
}

// doDigestRequest выполняет HTTP запрос с Digest-авторизацией
func doDigestRequest(client *http.Client, req *http.Request, username, password string) (*http.Response, error) {
	var originalBody []byte
	if req.Body != nil {
		var err error
		originalBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(originalBody))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	authHeader := resp.Header.Get("WWW-Authenticate")
	if authHeader == "" {
		return nil, fmt.Errorf("no WWW-Authenticate header")
	}

	params := parseDigestParams(authHeader)
	realm := params["realm"]
	nonce := params["nonce"]
	opaque := params["opaque"]
	algorithm := params["algorithm"]
	if algorithm == "" {
		algorithm = "MD5"
	}
	qop := params["qop"]

	if originalBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(originalBody))
	}

	uri := req.URL.RequestURI()
	nc := "00000001"
	cnonce := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))))

	var response string
	ha1 := md5Hash(fmt.Sprintf("%s:%s:%s", username, realm, password))
	ha2 := md5Hash(fmt.Sprintf("%s:%s", req.Method, uri))

	if qop == "auth" || qop == "auth-int" {
		response = md5Hash(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))
	} else {
		response = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	}

	auth := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		username, realm, nonce, uri, response)

	if algorithm != "" && algorithm != "MD5" {
		auth += fmt.Sprintf(`, algorithm="%s"`, algorithm)
	}
	if opaque != "" {
		auth += fmt.Sprintf(`, opaque="%s"`, opaque)
	}
	if qop != "" {
		auth += fmt.Sprintf(`, qop="%s", nc="%s", cnonce="%s"`, qop, nc, cnonce)
	}

	req.Header.Set("Authorization", auth)
	return client.Do(req)
}

// parseDigestParams парсит параметры из WWW-Authenticate заголовка
func parseDigestParams(header string) map[string]string {
	params := make(map[string]string)
	header = strings.TrimPrefix(header, "Digest ")
	header = strings.TrimPrefix(header, "digest ")
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(part[:eqIdx])
		value := strings.TrimSpace(part[eqIdx+1:])
		value = strings.Trim(value, `"`)
		params[key] = value
	}
	return params
}

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

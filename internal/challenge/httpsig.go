package challenge

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const webBotAuthTag = "web-bot-auth"

type parsedSignature struct {
	label      string
	components []componentID
	created    int64
	expires    int64
	keyID      string
	alg        string
	tag        string
	innerList  string
	signature  []byte
}

type componentID struct {
	name       string
	isDerived  bool
	raw        string
	hasReq     bool
	unofficial bool
}

func verifyHTTPMessageSignature(req *http.Request, public ed25519.PublicKey) error {
	inputHeader := strings.TrimSpace(req.Header.Get("Signature-Input"))
	sigHeader := strings.TrimSpace(req.Header.Get("Signature"))
	if inputHeader == "" || sigHeader == "" {
		return fmt.Errorf("missing signature headers")
	}

	inputs, err := parseSignatureInput(inputHeader)
	if err != nil {
		return err
	}
	signatures, err := parseSignatureHeader(sigHeader)
	if err != nil {
		return err
	}

	var chosen *parsedSignature
	for i := range inputs {
		candidate := inputs[i]
		if candidate.tag != webBotAuthTag {
			continue
		}
		rawSig, ok := signatures[candidate.label]
		if !ok {
			continue
		}
		candidate.signature = rawSig
		chosen = &candidate
		break
	}
	if chosen == nil {
		return fmt.Errorf("no web-bot-auth signature")
	}
	if chosen.alg != "" && !strings.EqualFold(chosen.alg, "ed25519") {
		return fmt.Errorf("unsupported signature algorithm")
	}
	if len(chosen.signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length")
	}

	base, err := signatureBase(req, chosen)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, base, chosen.signature) {
		return fmt.Errorf("ed25519 verification failed")
	}
	return nil
}

func signatureBase(req *http.Request, parsed *parsedSignature) ([]byte, error) {
	var b strings.Builder
	sawAuthority := false
	for _, component := range parsed.components {
		if component.unofficial {
			return nil, fmt.Errorf("unsupported signature component")
		}
		value, err := componentValue(req, component)
		if err != nil {
			return nil, err
		}
		if component.name == "@authority" {
			sawAuthority = true
			if !authorityMatches(req, value) {
				return nil, fmt.Errorf("@authority does not match host")
			}
		}
		b.WriteString(component.raw)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteByte('\n')
	}
	if !sawAuthority {
		return nil, fmt.Errorf("signed components must include @authority")
	}
	b.WriteString(`"@signature-params": `)
	b.WriteString(parsed.innerList)
	return []byte(b.String()), nil
}

func componentValue(req *http.Request, component componentID) (string, error) {
	if component.isDerived {
		switch component.name {
		case "@authority":
			return requestAuthority(req), nil
		case "@method":
			return strings.ToUpper(req.Method), nil
		case "@path":
			path := req.URL.EscapedPath()
			if path == "" {
				path = "/"
			}
			return path, nil
		case "@query":
			if req.URL.RawQuery == "" {
				return "?", nil
			}
			return "?" + req.URL.RawQuery, nil
		case "@target-uri":
			return requestTargetURI(req), nil
		case "@scheme":
			if req.TLS != nil {
				return "https", nil
			}
			return "http", nil
		default:
			return "", fmt.Errorf("unsupported derived component %q", component.name)
		}
	}

	if component.name == "" {
		return "", fmt.Errorf("empty component")
	}
	values := req.Header.Values(component.name)
	if len(values) == 0 {
		return "", fmt.Errorf("missing header %q", component.name)
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		trimmed = append(trimmed, strings.TrimSpace(value))
	}
	return strings.Join(trimmed, ", "), nil
}

func requestAuthority(req *http.Request) string {
	host := req.Host
	if host == "" && req.URL != nil {
		host = req.URL.Host
	}
	host = strings.ToLower(strings.TrimSpace(host))
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	if (req.TLS != nil && port == "443") || (req.TLS == nil && port == "80") {
		return hostname
	}
	return host
}

func authorityMatches(req *http.Request, signed string) bool {
	return strings.EqualFold(requestAuthority(req), strings.TrimSpace(signed))
}

func requestTargetURI(req *http.Request) string {
	if req.URL == nil {
		return ""
	}
	if req.URL.Scheme != "" && req.URL.Host != "" {
		return req.URL.String()
	}
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + requestAuthority(req) + req.URL.RequestURI()
}

func parseSignatureInput(header string) ([]parsedSignature, error) {
	members, err := splitDictionary(header)
	if err != nil {
		return nil, err
	}
	parsed := make([]parsedSignature, 0, len(members))
	for _, member := range members {
		label, value, ok := strings.Cut(member, "=")
		if !ok {
			return nil, fmt.Errorf("invalid Signature-Input member")
		}
		label = strings.TrimSpace(label)
		if !validSFKey(label) {
			return nil, fmt.Errorf("invalid signature label")
		}
		value = strings.TrimSpace(value)
		inner, params, err := splitInnerListAndParams(value)
		if err != nil {
			return nil, err
		}
		components, err := parseInnerListComponents(inner)
		if err != nil {
			return nil, err
		}
		item := parsedSignature{
			label:      label,
			components: components,
			innerList:  value,
		}
		for _, param := range params {
			name, raw, err := parseParameter(param)
			if err != nil {
				return nil, err
			}
			switch name {
			case "created":
				item.created, err = strconv.ParseInt(raw, 10, 64)
			case "expires":
				item.expires, err = strconv.ParseInt(raw, 10, 64)
			case "keyid":
				item.keyID, err = parseSFString(raw)
			case "alg":
				item.alg, err = parseSFString(raw)
			case "tag":
				item.tag, err = parseSFString(raw)
			default:
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		parsed = append(parsed, item)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("empty Signature-Input")
	}
	return parsed, nil
}

func parseSignatureHeader(header string) (map[string][]byte, error) {
	members, err := splitDictionary(header)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(members))
	for _, member := range members {
		label, value, ok := strings.Cut(member, "=")
		if !ok {
			return nil, fmt.Errorf("invalid Signature member")
		}
		label = strings.TrimSpace(label)
		raw, err := parseSFByteSequence(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		out[label] = raw
	}
	return out, nil
}

func parseSignatureAgent(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", fmt.Errorf("missing Signature-Agent")
	}
	// Structured Field String, optionally followed by parameters.
	if strings.HasPrefix(header, `"`) {
		value, rest, err := consumeSFString(header)
		if err != nil {
			return "", err
		}
		rest = strings.TrimSpace(rest)
		if rest != "" && !strings.HasPrefix(rest, ";") {
			return "", fmt.Errorf("invalid Signature-Agent")
		}
		return value, nil
	}
	return "", fmt.Errorf("Signature-Agent must be a structured string")
}

func splitInnerListAndParams(value string) (inner string, params []string, err error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "(") {
		return "", nil, fmt.Errorf("signature input is not an inner list")
	}
	depth := 0
	inString := false
	escape := false
	end := -1
	for i := 0; i < len(value); i++ {
		c := value[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
				i = len(value)
			}
		}
	}
	if end < 0 {
		return "", nil, fmt.Errorf("unterminated inner list")
	}
	inner = value[:end+1]
	rest := strings.TrimSpace(value[end+1:])
	if rest == "" {
		return inner, nil, nil
	}
	if !strings.HasPrefix(rest, ";") {
		return "", nil, fmt.Errorf("invalid signature parameters")
	}
	params, err = splitParameters(rest[1:])
	return inner, params, err
}

func parseInnerListComponents(inner string) ([]componentID, error) {
	inner = strings.TrimSpace(inner)
	if len(inner) < 2 || inner[0] != '(' || inner[len(inner)-1] != ')' {
		return nil, fmt.Errorf("invalid inner list")
	}
	body := strings.TrimSpace(inner[1 : len(inner)-1])
	if body == "" {
		return nil, fmt.Errorf("empty covered components")
	}
	items, err := splitListItems(body)
	if err != nil {
		return nil, err
	}
	components := make([]componentID, 0, len(items))
	for _, item := range items {
		component, err := parseComponentID(item)
		if err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	return components, nil
}

func parseComponentID(item string) (componentID, error) {
	item = strings.TrimSpace(item)
	name, rest, err := consumeSFString(item)
	if err != nil {
		return componentID{}, err
	}
	raw := strings.TrimSpace(item)
	component := componentID{
		name:      strings.ToLower(name),
		isDerived: strings.HasPrefix(name, "@"),
		raw:       raw,
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		component.raw = `"` + escapeSFString(component.name) + `"`
		return component, nil
	}
	if !strings.HasPrefix(rest, ";") {
		return componentID{}, fmt.Errorf("invalid component parameters")
	}
	params, err := splitParameters(rest[1:])
	if err != nil {
		return componentID{}, err
	}
	serialized := `"` + escapeSFString(component.name) + `"`
	for _, param := range params {
		pname, pvalue, err := parseParameter(param)
		if err != nil {
			return componentID{}, err
		}
		if pname == "req" && (pvalue == "" || pvalue == "?1") {
			component.hasReq = true
			serialized += ";req"
			continue
		}
		component.unofficial = true
		if pvalue == "" {
			serialized += ";" + pname
		} else {
			serialized += ";" + pname + "=" + pvalue
		}
	}
	component.raw = serialized
	return component, nil
}

func splitDictionary(header string) ([]string, error) {
	return splitTopLevel(header, ',')
}

func splitListItems(body string) ([]string, error) {
	return splitTopLevel(body, ' ')
}

func splitParameters(body string) ([]string, error) {
	return splitTopLevel(body, ';')
}

func splitTopLevel(value string, sep byte) ([]string, error) {
	var parts []string
	inString := false
	inBytes := false
	escape := false
	depth := 0
	start := 0
	for i := 0; i < len(value); i++ {
		c := value[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if inBytes {
			if c == ':' {
				inBytes = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case ':':
			inBytes = true
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return nil, fmt.Errorf("unbalanced )")
			}
			depth--
		default:
			if c == sep && depth == 0 {
				part := strings.TrimSpace(value[start:i])
				if part == "" {
					if sep == ' ' {
						start = i + 1
						continue
					}
					return nil, fmt.Errorf("empty structured-field member")
				}
				parts = append(parts, part)
				start = i + 1
			}
		}
	}
	if inString || inBytes || depth != 0 {
		return nil, fmt.Errorf("unterminated structured field")
	}
	tail := strings.TrimSpace(value[start:])
	if tail == "" {
		if sep == ' ' && len(parts) > 0 {
			return parts, nil
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty structured field")
		}
		return nil, fmt.Errorf("trailing separator")
	}
	return append(parts, tail), nil
}

func parseParameter(param string) (name, value string, err error) {
	param = strings.TrimSpace(param)
	name, value, ok := strings.Cut(param, "=")
	name = strings.ToLower(strings.TrimSpace(name))
	if !validSFKey(name) {
		return "", "", fmt.Errorf("invalid parameter name")
	}
	if !ok {
		return name, "", nil
	}
	return name, strings.TrimSpace(value), nil
}

func parseSFString(raw string) (string, error) {
	value, rest, err := consumeSFString(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rest) != "" {
		return "", fmt.Errorf("trailing data in string")
	}
	return value, nil
}

func consumeSFString(raw string) (value, rest string, err error) {
	raw = strings.TrimLeft(raw, " \t")
	if !strings.HasPrefix(raw, `"`) {
		return "", "", fmt.Errorf("expected structured-field string")
	}
	var b strings.Builder
	escape := false
	for i := 1; i < len(raw); {
		r, size := utf8.DecodeRuneInString(raw[i:])
		i += size
		if escape {
			if r != '"' && r != '\\' {
				return "", "", fmt.Errorf("invalid string escape")
			}
			b.WriteRune(r)
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			return b.String(), raw[i:], nil
		}
		if r < 0x20 {
			return "", "", fmt.Errorf("invalid string character")
		}
		b.WriteRune(r)
	}
	return "", "", fmt.Errorf("unterminated string")
}

func parseSFByteSequence(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != ':' || raw[len(raw)-1] != ':' {
		return nil, fmt.Errorf("invalid byte sequence")
	}
	payload := raw[1 : len(raw)-1]
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid byte sequence encoding")
	}
	return decoded, nil
}

func escapeSFString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func quoteSFString(value string) string {
	return `"` + escapeSFString(value) + `"`
}

func validSFKey(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	first := value[0]
	if first != '*' && (first < 'a' || first > 'z') {
		return false
	}
	for i := 1; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '_', '-', '.', '*':
			continue
		default:
			return false
		}
	}
	return true
}

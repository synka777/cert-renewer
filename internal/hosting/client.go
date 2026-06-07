package hosting

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

const (
	baseURL      = "https://1984.hosting"
	loginPath    = "/accounts/login/"
	dnsEntryPath = "/domains/entry/"
)

type Client struct {
	http     *http.Client
	username string
	password string
}

func NewClient(username, password string) (*Client, error) {
	// cookiejar.New(nil) creates an in-memory store that automatically saves cookies
	// from responses and re-sends them on subsequent requests to the same domain for automatic authentication
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	return &Client{
		http: &http.Client{
			Jar: jar,
		},
		username: username,
		password: password,
	}, nil
}

func (c *Client) Login() error {
	// First GET to obtain the CSRF token from the login page
	resp, err := c.http.Get(baseURL + loginPath)
	if err != nil {
		return fmt.Errorf("fetching login page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading login page body: %w", err)
	}

	csrf, err := extractCSRF(body)
	if err != nil {
		return fmt.Errorf("extracting CSRF token: %w", err)
	}

	log.Printf("obtained CSRF token: %s", csrf)

	// POST credentials + CSRF token to authenticate
	fields := url.Values{
		"username":            {c.username},
		"password":            {c.password},
		"csrfmiddlewaretoken": {csrf},
	}

	req, err := http.NewRequest("POST", baseURL+loginPath, strings.NewReader(fields.Encode()))
	if err != nil {
		return fmt.Errorf("building login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", baseURL+loginPath)

	resp2, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posting login form: %w", err)
	}
	defer resp2.Body.Close()

	// 1984 redirects to "/" on success; anything else means bad credentials
	if resp2.Request.URL.Path == loginPath {
		return fmt.Errorf("login failed: still on login page, check uour credentials")
	}

	log.Printf("login successful")
	return nil
}

func (c *Client) AddTXTRecord(domain, value string) error {
	csrf, err := c.fetchDNSPageCSRF(domain)
	if err != nil {
		return err
	}

	fields := url.Values{
		"csrfmiddlewaretoken": {csrf},
		"type":                {"TXT"},
		"dnsname":             {"_acme-challenge"},
		"dnsdata":             {value},
		"ttl":                 {"60"},
	}

	return c.postDNSEntry(domain, fields)
}

func (c *Client) DeleteTXTRecord(domain, recordID string) error {
	csrf, err := c.fetchDNSPageCSRF(domain)
	if err != nil {
		return err
	}

	fields := url.Values{
		"csrfmiddlewaretoken": {csrf},
		"delete":              {"true"},
		"record_id":           {recordID},
	}

	return c.postDNSEntry(domain, fields)
}

func (c *Client) fetchDNSPageCSRF(domain string) (string, error) {
	resp, err := c.http.Get(baseURL + dnsEntryPath + domain + "/")
	if err != nil {
		return "", fmt.Errorf("fetching DNS page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading DNS page body %w", err)
	}

	csrf, err := extractCSRF(body)
	if err != nil {
		return "", fmt.Errorf("extracting CSRF from DNS page: %w", err)
	}

	return csrf, nil
}

// (c *Client) is a method receiver, these are Go's way of attaching methods to types.
// c here is the receiver — roughly equivalent to "this" in other languages
// The *Client (pointer receiver) means the method operates on the actual Client in memory, not a copy;
// important here since http.Client has internal state we don't want copied.
func (c *Client) postDNSEntry(domain string, fields url.Values) error {
	req, err := http.NewRequest("POST", baseURL+dnsEntryPath+domain+"/", strings.NewReader(fields.Encode()))
	if err != nil {
		return fmt.Errorf("building DNS entry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", baseURL+dnsEntryPath+domain+"/")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posting DNS entry: %w:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from DNS enry point", resp.StatusCode)
	}
	return nil
}

func extractCSRF(body []byte) (string, error) {
	// The server embeds a secret token in the HTML of any page with a form.
	// You must send that token back with your POST, proving you actually loaded the page first.
	// It prevents cross-site request forgery attacks, that's why every mutating operation here is two steps:
	// GET the page to grab the token, then POST with it.
	re := regexp.MustCompile(`name="csrfmiddlewaretoken"\s+value="([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("CSRF token not found in page")
	}
	return string(matches[1]), nil
}

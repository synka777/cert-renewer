package hosting

import (
	"encoding/json"
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
	// First GET the login page to get the CSRF cookie set
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

	// POST to the JSON API endpoint, just like the browser JS does
	fields := url.Values{
		"username": {c.username},
		"password": {c.password},
		"otpkey":   {""},
	}

	req, err := http.NewRequest("POST", baseURL+"/api/auth/", strings.NewReader(fields.Encode()))
	if err != nil {
		return fmt.Errorf("building login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", baseURL+loginPath)
	req.Header.Set("X-CSRFToken", csrf)

	resp2, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posting to auth API: %w", err)
	}
	defer resp2.Body.Close()

	var result struct {
		LoggedIn bool     `json:"loggedin"`
		Error    []string `json:"error"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding auth response: %w", err)
	}

	if !result.LoggedIn {
		if len(result.Error) > 0 {
			return fmt.Errorf("login failed: %s", strings.Join(result.Error, ", "))
		}
		return fmt.Errorf("login failed: unknown reason")
	}

	log.Printf("login successful")
	return nil
}

func (c *Client) getCSRFFromCookie() (string, error) {
	u, _ := url.Parse(baseURL)
	for _, cookie := range c.http.Jar.Cookies(u) {
		if cookie.Name == "csrftoken" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("csrftoken cookie not found")
}

func (c *Client) AddTXTRecord(domain, value string) error {
	csrf, err := c.getCSRFFromCookie()
	if err != nil {
		return err
	}

	fields := url.Values{
		"entry": {"new"},
		"type":  {"TXT"},
		"ttl":   {"900"},
		"zone":  {domain},
		"host":  {"_acme-challenge"},
		"value": {value},
	}

	return c.postDNSEntry(domain, csrf, fields)
}

func (c *Client) DeleteTXTRecord(domain, value string) error {
	csrf, err := c.getCSRFFromCookie()
	if err != nil {
		return err
	}

	fields := url.Values{
		"entry": {"delete"},
		"type":  {"TXT"},
		"zone":  {domain},
		"host":  {"_acme-challenge"},
		"value": {value},
	}

	return c.postDNSEntry(domain, csrf, fields)
}

func (c *Client) postDNSEntry(domain, csrf string, fields url.Values) error {
	req, err := http.NewRequest("POST", baseURL+dnsEntryPath, strings.NewReader(fields.Encode()))
	if err != nil {
		return fmt.Errorf("building DNS entry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", baseURL+"/domains/"+domain+"/")
	req.Header.Set("X-CSRFToken", csrf)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posting DNS entry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from DNS entry endpoint", resp.StatusCode)
	}

	return nil
}

func extractCSRF(body []byte) (string, error) {
	re := regexp.MustCompile(`name="csrfmiddlewaretoken"\s+value="([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("CSRF token not found in page")
	}
	return string(matches[1]), nil
}

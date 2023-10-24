package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RefreshInterval   = 30 * time.Second
	RetryInterval     = 5 * time.Second
	CooldownInterval  = 5 * time.Minute
	Retries           = 3
	DetectionUrl      = "http://captive.apple.com/"
	ExpectedPortalUrl = "https://wlan.hs-heilbronn.de/login.html"
	Network           = "internet"
	Username          = "gast"
	Password          = "gast"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

var lazyClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Timeout: 5 * time.Second,
}

func main() {
	log.Println("HHN WiFi captive portal keepalive daemon")
	log.Println("Network:", Network)
	log.Println("Login:", Username, ":", Password)
	log.Println("Refresh:", RefreshInterval)
	log.Println("If you use this tool, you accept the terms of service")
	time.Sleep(2 * time.Second)

	tries := 0
	for {
		if keepalive() {
			time.Sleep(RefreshInterval)
			continue
		}

		tries++
		if tries >= Retries {
			log.Println("Could not log in, delaying for", CooldownInterval)
			time.Sleep(CooldownInterval)
			tries = 0
			continue
		}

		log.Println("Previous log-in failed, retrying in", RetryInterval, "for", Retries-tries, "more time(s)")
		time.Sleep(RetryInterval)
	}
}

func keepalive() (online bool) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	log.Println("Detecting portals")
	portal, err := detectPortal()
	if err != nil {
		log.Println(err)
		return
	}
	if portal == "" {
		log.Println("No portal detected")
		return true
	}

	log.Println("Portal detected, redirecting to", portal)
	if !strings.HasPrefix(portal, ExpectedPortalUrl) {
		log.Println("Not the expected HHN portal at", ExpectedPortalUrl)
		return
	}

	log.Println("Submitting log-in request")
	err = solvePortal()
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Log-in request received successfully")
	return true
}

func detectPortal() (portal string, err error) {
	req, err := http.NewRequest(http.MethodGet, DetectionUrl, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", UserAgent)
	resp, err := lazyClient.Do(req)
	if err != nil {
		return
	}

	err = resp.Body.Close()
	portal = resp.Header.Get("Location")
	return
}

func solvePortal() (err error) {
	form := url.Values{}
	form.Set("buttonClicked", "4")
	form.Set("err_flag", "0")
	form.Set("err_msg", "")
	form.Set("info_flag", "0")
	form.Set("info_msg", "")
	form.Set("redirect_url", DetectionUrl)
	form.Set("network_name", "Guest Network")
	form.Set("username", Username)
	form.Set("password", Password)

	req, err := http.NewRequest(http.MethodPost, ExpectedPortalUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", ExpectedPortalUrl)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		_ = resp.Body.Close()
		return
	}

	err = resp.Body.Close()
	if err != nil {
		return
	}

	if (resp.StatusCode / 100) != 2 {
		err = fmt.Errorf("received bad status %s", resp.Status)
	}
	return
}

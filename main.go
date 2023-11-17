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
	Version            = "1.3"
	IdleInterval       = 25 * time.Minute
	RefreshInterval    = 30 * time.Second
	RetryInterval      = 10 * time.Second
	CooldownInterval   = 5 * time.Minute
	Retries            = 4
	DetectionUrl       = "http://1.1.1.3/"
	ExpectedThroughUrl = "https://1.1.1.3/"
	ExpectedPortalUrl  = "https://wlan.hs-heilbronn.de/login.html"
	Network            = "internet"
	Username           = "gast"
	Password           = "gast"
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
	log.Println("Version:", Version)
	log.Println("Network:", Network)
	log.Println("Login:", Username, ":", Password)
	log.Println("If you use this tool, you accept the terms of service")
	time.Sleep(2 * time.Second)

	tries := 0
	for {
		online, solvable := testConnection()
		if online {
			if tries > 0 {
				tries = 0
				log.Println("Repair successful, now idling for", IdleInterval)
				time.Sleep(IdleInterval)
				log.Println("Idle time elapsed, now refreshing every", RefreshInterval)
			}

			time.Sleep(RefreshInterval)
			continue
		}

		tries++
		if tries > Retries {
			log.Println("Checks temporarily suspended, continuing in", CooldownInterval)
			time.Sleep(CooldownInterval)
			tries = 0
			continue
		}

		if solvable && repairConnection() {
			continue
		}

		log.Println("Not connected, checking again in", RetryInterval, "for", Retries-tries+1, "more time(s)")
		time.Sleep(RetryInterval)
	}
}

func testConnection() (online bool, solvable bool) {
	defer func() {
		if err := recover(); err != nil {
			online, solvable = false, false
			log.Println(err)
		}
	}()

	log.Println("Testing connection and detecting portals")
	redirect, err := detectPortal()
	if err != nil {
		log.Println(err)
		return
	}
	if redirect == "" || strings.HasPrefix(redirect, ExpectedThroughUrl) {
		log.Println("Connected and no portal detected")
		online = true
		return
	}

	log.Println("Portal detected, redirecting to", redirect)
	if strings.HasPrefix(redirect, ExpectedPortalUrl) {
		solvable = true
		return
	}

	log.Println("Not the expected HHN portal at", ExpectedPortalUrl)
	return
}

func repairConnection() (attempted bool) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	log.Println("Attempting repair by submitting log-in request")
	err := solvePortal()
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Log-in request received successfully")
	return true
}

func detectPortal() (redirect string, err error) {
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
	redirect = resp.Header.Get("Location")
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

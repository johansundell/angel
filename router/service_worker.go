package router

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// offlinePage is what the service worker shows when a page can't be loaded.
const offlinePage = "/assets/offline.html"

// appShell is everything the service worker stores on the device: the files
// the offline page needs and the install icons. Pages behind a PIN are never
// stored, so a Daily Note doesn't stay on a phone (ADR-0007).
var appShell = []string{
	"/assets/css/main.css",
	"/assets/img/logo.svg",
	"/assets/img/favicon.svg",
	"/assets/img/favicon.png",
	"/assets/img/apple-touch-icon.png",
	"/assets/img/icon-192.png",
	"/assets/img/icon-512.png",
	"/assets/img/icon-maskable-512.png",
	"/manifest.webmanifest",
	offlinePage,
}

// serviceWorkerSource is filled in with the build version, the app shell and
// the offline page. The version makes each release a new worker, which the
// browser installs in place of the old one along with a fresh cache.
const serviceWorkerSource = `"use strict";
const VERSION = {{VERSION}};
const CACHE = "angel-" + VERSION;
const PRECACHE = {{PRECACHE}};
const OFFLINE = {{OFFLINE}};

self.addEventListener("install", function (event) {
	event.waitUntil(
		caches.open(CACHE)
			.then(function (cache) {
				return cache.addAll(PRECACHE.map(function (url) { return new Request(url, { cache: "reload" }); }));
			})
			.then(function () { return self.skipWaiting(); })
	);
});

self.addEventListener("activate", function (event) {
	event.waitUntil(
		caches.keys()
			.then(function (keys) {
				return Promise.all(keys.filter(function (key) { return key !== CACHE; }).map(function (key) { return caches.delete(key); }));
			})
			.then(function () { return self.clients.claim(); })
	);
});

// Everything is fetched from the network and nothing new is stored. Page
// loads that fail show the offline page; app-shell files fall back to the
// copy stored at install so the offline page has its styling and logo. All
// other requests (POSTs, the dashboard's poll, the health check and the
// logs) are left to the browser.
self.addEventListener("fetch", function (event) {
	const req = event.request;
	if (req.method !== "GET") return;
	const url = new URL(req.url);
	if (url.origin !== self.location.origin) return;
	if (url.pathname === "/healthz" || url.pathname.startsWith("/logs/")) return;

	if (req.mode === "navigate") {
		event.respondWith(fetch(req).catch(function () {
			return caches.match(OFFLINE, { cacheName: CACHE });
		}));
		return;
	}
	if (PRECACHE.includes(url.pathname)) {
		event.respondWith(fetch(req).catch(function () {
			return caches.match(url.pathname, { cacheName: CACHE });
		}));
	}
});
`

// serviceWorker serves the worker from the site root, so its scope covers
// every page. no-cache makes the browser check for a new release each time.
func serviceWorker(version string) (gin.HandlerFunc, error) {
	enc := func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	}
	v, err := enc(version)
	if err != nil {
		return nil, err
	}
	shell, err := enc(appShell)
	if err != nil {
		return nil, err
	}
	offline, err := enc(offlinePage)
	if err != nil {
		return nil, err
	}
	src := strings.NewReplacer("{{VERSION}}", v, "{{PRECACHE}}", shell, "{{OFFLINE}}", offline).Replace(serviceWorkerSource)

	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/javascript; charset=utf-8", []byte(src))
	}, nil
}

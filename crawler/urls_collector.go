package crawler

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

type SearchEngine struct {
	Name           string
	URLTemplate    string
	NextSelectors  []string
	ResultSelector string
}

type Browser struct {
	logger           *zap.Logger
	proxyURL         string
	SupportedEngines []SearchEngine
	ChromedpOptions  []chromedp.ExecAllocatorOption
}

func NewBrowser(logger *zap.Logger, torProxyURL string) *Browser {
	return &Browser{
		logger:   logger,
		proxyURL: torProxyURL,
		SupportedEngines: []SearchEngine{
			{
				Name:        "DuckDuckGo",
				URLTemplate: "https://duckduckgo.com/?q=%s",
				NextSelectors: []string{
					"button[aria-label='More results']",
					".more-results",
					"[data-testid='more-results']",
				},
				ResultSelector: "a[data-testid='result-title-a']",
			},
			{
				Name:        "Bing",
				URLTemplate: "https://www.bing.com/search?q=%s",
				NextSelectors: []string{
					"a[aria-label='Next page']",
					".sb_pagN",
					"a[title='Next page']",
				},
				ResultSelector: ".b_algo h2 a",
			},
			{
				Name:        "Google",
				URLTemplate: "https://www.google.com/search?q=%s",
				NextSelectors: []string{
					"a[aria-label='Next page']",
					"a#pnnext",
					"[aria-label='Next']",
				},
				ResultSelector: "h3 a",
			},
		},
		ChromedpOptions: append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),

			// Stealth options
			chromedp.Flag("accept-language", "en-US,en;q=0.9"),
			chromedp.Flag("accept-encoding", "gzip, deflate, br"),
			chromedp.Flag("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("exclude-switches", "enable-automation"),
			chromedp.Flag("disable-extensions", ""),
		),
	}
}

func (b *Browser) CollectUrls(ctx context.Context, query string) ([]string, error) {
	// ================
	// Browser Context
	// ================
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, b.ChromedpOptions...)
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)

	cancel := func() {
		taskCancel()
		allocCancel()
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(taskCtx, 500*time.Second)
	oldCancel := cancel
	cancel = func() {
		timeoutCancel()
		oldCancel()
	}
	defer cancel()
	taskCtx = timeoutCtx

	// ================
	// Doing Search
	// ================
	engine1 := b.SupportedEngines[0]
	searchURL := fmt.Sprintf(engine1.URLTemplate, url.QueryEscape(query))

	b.logger.Info("Navigating to search",
		zap.String("url", searchURL),
		zap.String("engine", engine1.Name))

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitVisible("body"),
		chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', {
				get: () => undefined,
			});
			window.chrome = { runtime: {} };
			Object.defineProperty(navigator, 'plugins', {
				get: () => [1, 2, 3, 4, 5],
			});
		`, nil),
	)
	if err != nil {
		b.logger.Info("")
	}

	var rawLinks []map[string]string
	script := fmt.Sprintf(`
		let links = Array.from(document.querySelectorAll('%s')).map(link => ({
			href: link.href,
			text: link.textContent.trim()
		}));
		
		if (links.length === 0) {
			links = Array.from(document.querySelectorAll('a[href]')).map(link => ({
				href: link.href,
				text: link.textContent.trim()
			}));
		}
		
		links.filter(link => 
			link.href && 
			!link.href.startsWith('javascript:') &&
			link.href.startsWith('http') &&
			link.text.length > 0
		)
	`, engine1.ResultSelector)
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(script, &rawLinks),
	)

	if err != nil {
		b.logger.Info("")
	}

	var results []string
	for i, link := range rawLinks {
		results = append(results, link["href"])
		// Log first few for debugging
		if i < 3 {
			b.logger.Info("Extracted search result",
				zap.String("url", link["href"]),
				zap.String("title", link["text"][:200]))
		}
	}

	return results, err
}

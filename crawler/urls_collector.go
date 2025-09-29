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
					`button[id="more-results"]`,
				},
				ResultSelector: `section[data-testid="mainline"]`,
			},
			{
				Name:        "Brave",
				URLTemplate: "https://search.brave.com/search?q=%s",
				NextSelectors: []string{
					`a.button[role="link"]`,
				},
				ResultSelector: `div#results`,
			},
			{
				Name:        "Startpage",
				URLTemplate: "https://www.startpage.com/sp/search?q=%s",
				NextSelectors: []string{
					`button[data-testid="pagination-button"][type="submit"]`,
				},
				ResultSelector: `section#main`,
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
		b.logger.Error("Failed to navigate and setup page", zap.Error(err))

		// Log current page state for debugging
		var currentURL, title, domHTML string
		chromedp.Run(taskCtx,
			chromedp.Location(&currentURL),
			chromedp.Title(&title),
			chromedp.OuterHTML("html", &domHTML),
		)

		b.logger.Info("Page state after navigation error",
			zap.String("current_url", currentURL),
			zap.String("title", title),
			zap.Int("dom_length", len(domHTML)))

		// Log first 1000 chars of DOM for debugging
		if len(domHTML) > 1000 {
			b.logger.Debug("DOM snippet", zap.String("html", domHTML[:1000]+"..."))
		} else {
			b.logger.Debug("Full DOM", zap.String("html", domHTML))
		}

		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	// Wait a bit and then log page state before extraction
	time.Sleep(2 * time.Second)

	var currentURL, title, domHTML string
	err = chromedp.Run(taskCtx,
		chromedp.Location(&currentURL),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &domHTML),
	)
	if err != nil {
		b.logger.Error("Failed to get page state", zap.Error(err))
	} else {
		b.logger.Info("Page state before extraction",
			zap.String("current_url", currentURL),
			zap.String("title", title),
			zap.Int("dom_length", len(domHTML)))
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
			link.href.startsWith('https') &&
			link.text.length > 0
		)
	`, engine1.ResultSelector)

	err = chromedp.Run(taskCtx,
		chromedp.Evaluate(script, &rawLinks),
	)

	if err != nil {
		b.logger.Error("Failed to extract links",
			zap.Error(err),
			zap.String("selector", engine1.ResultSelector),
			zap.String("script", script))

		// Log DOM again after extraction failure
		var postErrorHTML string
		chromedp.Run(taskCtx, chromedp.OuterHTML("html", &postErrorHTML))

		b.logger.Info("DOM state after extraction error",
			zap.Int("dom_length", len(postErrorHTML)))

		// Log DOM snippet for debugging
		if len(postErrorHTML) > 2000 {
			b.logger.Debug("DOM snippet after error", zap.String("html", postErrorHTML[:2000]+"..."))
		} else {
			b.logger.Debug("Full DOM after error", zap.String("html", postErrorHTML))
		}

		return nil, fmt.Errorf("link extraction failed: %w", err)
	}

	b.logger.Info("Successfully extracted links",
		zap.Int("total_links", len(rawLinks)),
		zap.String("selector_used", engine1.ResultSelector))

	var results []string
	for i, link := range rawLinks {
		results = append(results, link["href"])
		// Log first few for debugging
		if i < 3 {
			linkText := link["text"]
			if len(linkText) > 200 {
				linkText = linkText[:200]
			}
			b.logger.Info("Extracted search result",
				zap.String("url", link["href"]),
				zap.String("title", linkText))
		}
	}

	// Final DOM logging if we want to see the successful state
	if len(results) == 0 {
		b.logger.Warn("No results found, logging final DOM state")
		var finalHTML string
		chromedp.Run(taskCtx, chromedp.OuterHTML("html", &finalHTML))

		if len(finalHTML) > 3000 {
			b.logger.Debug("Final DOM snippet", zap.String("html", finalHTML[:3000]+"..."))
		} else {
			b.logger.Debug("Final DOM", zap.String("html", finalHTML))
		}
	}

	return results, err
}

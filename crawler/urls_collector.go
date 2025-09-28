package crawler

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

func (w *Crawler) CollectUrls(ctx context.Context, query string) []string {
	w.logger.Info("CollectUrls started", zap.String("query", query))

	var urls []string

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,

		// More realistic user agent
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),

		// Additional headers to appear more legitimate
		chromedp.Flag("accept-language", "en-US,en;q=0.9"),
		chromedp.Flag("accept-encoding", "gzip, deflate, br"),
		chromedp.Flag("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"),

		// Disable automation indicators
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-extensions", ""),
	)
	opts = append(opts, chromedp.ProxyServer(w.torProxyUrl))

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer func() {
		cancel()
		w.logger.Info("Cancelled ExecAllocator context")
	}()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer func() {
		cancel()
		w.logger.Info("Cancelled ChromeDP task context")
	}()

	taskCtx, cancel = context.WithTimeout(taskCtx, 120*time.Minute)
	defer func() {
		cancel()
		w.logger.Info("Cancelled timeout context")
	}()

	strategies := []struct {
		name string
		url  func(string) string
	}{
		// {
		// 	name: "DuckDuckGo",
		// 	url:  func(q string) string { return fmt.Sprintf("https://duckduckgo.com/?q=%s", url.QueryEscape(q)) },
		// },
		// {
		// 	name: "Bing",
		// 	url:  func(q string) string { return fmt.Sprintf("https://www.bing.com/search?q=%s", url.QueryEscape(q)) },
		// },
		{
			name: "StartPage",
			url: func(q string) string {
				return fmt.Sprintf("https://www.startpage.com/search?query=%s", url.QueryEscape(q))
			},
		},
	}

	for _, strategy := range strategies {
		searchURL := strategy.url(query)
		w.logger.Info("Built search URL", zap.String("searchURL", searchURL), zap.String("strategy", strategy.name))

		var pageTitle string
		var bodyHTML string
		var isCaptcha bool
		var links []map[string]string

		err := chromedp.Run(taskCtx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				// Add some randomness to appear more human
				time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)
				return nil
			}),

			chromedp.Navigate(searchURL),
			chromedp.WaitVisible("body", chromedp.ByQuery),

			// Add stealth JavaScript to hide automation indicators
			chromedp.Evaluate(`
				Object.defineProperty(navigator, 'webdriver', {
					get: () => undefined,
				});
				window.chrome = { runtime: {} };
				Object.defineProperty(navigator, 'plugins', {
					get: () => [1, 2, 3, 4, 5],
				});
			`, nil),

			chromedp.Sleep(3*time.Second), // Wait for page to fully load
			chromedp.Title(&pageTitle),
			chromedp.Evaluate(`document.body.innerHTML.substring(0, 1000)`, &bodyHTML),

			// Check for CAPTCHA indicators
			chromedp.Evaluate(`
				document.body.innerHTML.toLowerCase().includes('captcha') ||
				document.body.innerHTML.toLowerCase().includes('recaptcha') ||
				document.querySelector('#captcha-form') !== null ||
				document.querySelector('.g-recaptcha') !== null
			`, &isCaptcha),
		)

		w.logger.Info("Page inspection results",
			zap.String("strategy", strategy.name),
			zap.String("title", pageTitle),
			zap.Bool("isCaptcha", isCaptcha),
			zap.String("bodyPreview", bodyHTML[:200]),
		)

		if err != nil {
			w.logger.Error("Error during page inspection", zap.String("strategy", strategy.name), zap.Error(err))
			continue
		}

		if isCaptcha {
			w.logger.Warn("CAPTCHA detected, trying next strategy", zap.String("strategy", strategy.name))
			continue
		}

		// Extract links based on search engine
		var extractionScript string
		switch strategy.name {
		case "Bing":
			extractionScript = `
				Array.from(document.querySelectorAll('.b_algo h2 a, .b_title a')).map(link => ({
					href: link.href,
					text: link.textContent.trim()
				}))
			`
		case "StartPage":
			extractionScript = `
				Array.from(document.querySelectorAll('.w-gl__result-title, .result-title')).map(link => ({
					href: link.href,
					text: link.textContent.trim()
				}))
			`
		default: // DuckDuckGo
			extractionScript = `
				Array.from(document.querySelectorAll('a[data-testid="result-title-a"], .result__url')).map(link => ({
					href: link.href,
					text: link.textContent.trim()
				}))
			`
		}

		// Add universal link extraction as fallback
		extractionScript = `
			let specificLinks = ` + extractionScript + `;
			
			if (specificLinks.length === 0) {
				specificLinks = Array.from(document.querySelectorAll('a[href]')).map(link => ({
					href: link.href,
					text: link.textContent.trim()
				}));
			}
			
			specificLinks.filter(link => 
				link.href && 
				!link.href.startsWith('javascript:') &&
				!link.href.includes('/search') &&
				!link.href.includes('/url?') &&
				!link.href.includes('webcache.googleusercontent.com') &&
				!link.href.includes('duckduckgo.com') &&
				!link.href.includes('bing.com') &&
				!link.href.includes('startpage.com') &&
				link.href.startsWith('http') &&
				link.text.length > 0
			)
		`

		err = chromedp.Run(taskCtx,
			chromedp.Sleep(2*time.Second), // Additional wait for dynamic content
			chromedp.Evaluate(extractionScript, &links),
		)

		if err != nil {
			w.logger.Error("Error extracting links", zap.String("strategy", strategy.name), zap.Error(err))
			continue
		}

		w.logger.Info("Successfully extracted links",
			zap.String("strategy", strategy.name),
			zap.Int("count", len(links)),
		)

		if len(links) > 0 {
			seenUrls := make(map[string]bool)

			for _, link := range links {
				href := link["href"]

				if href == "" {
					continue
				}

				parsedUrl, err := url.Parse(href)
				if err != nil {
					w.logger.Warn("Failed to parse URL", zap.String("href", href), zap.Error(err))
					continue
				}

				pu := parsedUrl.String()
				if seenUrls[pu] {
					continue
				}

				urls = append(urls, pu)
				seenUrls[pu] = true
				w.logger.Info("Added URL", zap.String("url", pu), zap.String("strategy", strategy.name))
			}

			w.logger.Info("Successfully collected URLs",
				zap.Int("total", len(urls)),
				zap.String("query", query),
				zap.String("successful_strategy", strategy.name),
			)
			return urls
		}
	}

	w.logger.Warn("All search strategies failed or returned no results")
	return urls
}

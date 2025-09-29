package crawler

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

type SearchEngine struct {
	Name             string
	URLTemplate      string
	NextPageSelector string
	ResultSelector   string
}

type Browser struct {
	logger           *zap.Logger
	proxyURL         string
	SupportedEngines []SearchEngine
	ChromedpOptions  []chromedp.ExecAllocatorOption

	// Pagination state
	maxPages         int
	currentPage      int
	pageDelay        time.Duration
	allCollectedURLs []string
}

func NewBrowser(logger *zap.Logger, torProxyURL string) *Browser {
	return &Browser{
		logger:   logger,
		proxyURL: torProxyURL,
		SupportedEngines: []SearchEngine{
			{
				Name:             "Brave",
				URLTemplate:      "https://search.brave.com/search?q=%s",
				NextPageSelector: `a.button[role="link"][rel="noopener"]`,
				ResultSelector:   `#results`,
			},
			{
				Name:             "Startpage",
				URLTemplate:      "https://www.startpage.com/sp/search?q=%s",
				NextPageSelector: `form[aria-label="go to page Next"] button[data-testid="pagination-button"]`,
				ResultSelector:   `section#main`,
			},
		},
		ChromedpOptions: append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),

			// Your existing stealth options
			chromedp.Flag("accept-language", "en-US,en;q=0.9"),
			chromedp.Flag("accept-encoding", "gzip, deflate, br"),
			chromedp.Flag("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("exclude-switches", "enable-automation"),
			chromedp.Flag("disable-extensions", ""),
		),
		maxPages:         50,
		currentPage:      0,
		pageDelay:        time.Second * 2,
		allCollectedURLs: []string{},
	}
}

func (b *Browser) CollectUrls(ctx context.Context, query string) ([]string, error) {
	engine := b.SupportedEngines[1]

	taskCtx, cancel, err := b.setupBrowserContext(ctx, time.Minute*5)
	if err != nil {
		return nil, fmt.Errorf("failed to setup browser context: %w", err)
	}
	defer cancel()

	searchURL := fmt.Sprintf(engine.URLTemplate, url.QueryEscape(query))
	if err := b.navigateToPage(taskCtx, searchURL, engine.Name); err != nil {
		return nil, fmt.Errorf("failed to navigate to first page: %w", err)
	}

	for b.currentPage < b.maxPages {
		b.currentPage++

		b.logger.Info("Processing page",
			zap.Int("current_page", b.currentPage),
			zap.Int("max_pages", b.maxPages),
			zap.String("engine", engine.Name))

		if err := b.checkPageState(taskCtx); err != nil {
			b.logger.Warn("Page state check failed",
				zap.Error(err),
				zap.Int("page", b.currentPage))
		}

		urls, err := b.extractLinksFromCurrentPage(taskCtx, engine)
		if err != nil {
			b.logger.Error("Failed to extract links from page",
				zap.Error(err),
				zap.Int("page", b.currentPage))
		} else {
			b.allCollectedURLs = append(b.allCollectedURLs, urls...)
			b.logger.Info("Collected URLs from page",
				zap.Int("page", b.currentPage),
				zap.Int("urls_this_page", len(urls)),
				zap.Int("total_urls", len(b.allCollectedURLs)),
				zap.Strings("urls", urls),
			)
		}

		if b.currentPage >= b.maxPages {
			b.logger.Info("Reached maximum pages", zap.Int("max_pages", b.maxPages))
			break
		}

		// b.logDOMBeforeNextPage(taskCtx, engine)

		hasNext, err := b.goToNextPage(taskCtx, engine)
		if err != nil {
			b.logger.Error("Failed to navigate to next page",
				zap.Error(err),
				zap.Int("current_page", b.currentPage))
			break
		}

		if !hasNext {
			b.logger.Info("No more pages available", zap.Int("final_page", b.currentPage))
			break
		}

		if b.pageDelay > 0 {
			b.logger.Debug("Waiting between pages", zap.Duration("delay", b.pageDelay))
			time.Sleep(b.pageDelay)
		}
	}

	b.logger.Info("Crawling completed",
		zap.Int("total_pages", b.currentPage),
		zap.Int("total_urls", len(b.allCollectedURLs)),
		zap.String("engine", engine.Name))

	return b.allCollectedURLs, nil
}

func (b *Browser) setupBrowserContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, b.ChromedpOptions...)
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)

	cancel := func() {
		taskCancel()
		allocCancel()
	}

	if timeout > 0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(taskCtx, timeout)
		oldCancel := cancel
		cancel = func() {
			timeoutCancel()
			oldCancel()
		}
		taskCtx = timeoutCtx
	}

	return taskCtx, cancel, nil
}

func (b *Browser) navigateToPage(ctx context.Context, url, engineName string) error {
	b.logger.Info("Navigating to page",
		zap.String("url", url),
		zap.String("engine", engineName))

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)

	if err != nil {
		b.logger.Error("Navigation failed",
			zap.Error(err),
			zap.String("url", url))
		return fmt.Errorf("navigation failed: %w", err)
	}

	return nil
}

func (b *Browser) checkPageState(ctx context.Context) error {
	var currentURL, title, readyState string

	err := chromedp.Run(ctx,
		chromedp.Location(&currentURL),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.readyState`, &readyState),
	)

	if err != nil {
		return fmt.Errorf("failed to get page state: %w", err)
	}

	b.logger.Info("Page state",
		zap.String("url", currentURL),
		zap.String("title", title),
		zap.String("ready_state", readyState),
		zap.Int("page", b.currentPage))

	// Check for common blocking scenarios
	if title == "Access Denied" || title == "Blocked" {
		return fmt.Errorf("page access blocked: %s", title)
	}

	return nil
}

func (b *Browser) extractLinksFromCurrentPage(ctx context.Context, engine SearchEngine) ([]string, error) {
	var rawLinks []map[string]string

	script := fmt.Sprintf(`
		let resultsDiv = document.querySelector('%s');
		let links = [];
		
		if (resultsDiv) {
			links = Array.from(resultsDiv.querySelectorAll('a[href]')).map(link => ({
				href: link.href,
				text: link.textContent.trim()
			}));
		}
		
		// Fallback to all links if no results div found
		if (links.length === 0) {
			links = Array.from(document.querySelectorAll('a[href]')).map(link => ({
				href: link.href,
				text: link.textContent.trim()
			}));
		}
		
		links = links.filter(link => 
			link.href &&
			!link.href.startsWith('javascript:') &&
			link.href.startsWith('https') &&
			link.text.length > 0 &&
			!link.href.includes('search.brave.com') &&
			!link.href.includes('duckduckgo.com') &&
			!link.href.includes('startpage.com')
		);

		// ✅ Deduplicate by href
		links = Array.from(new Map(links.map(link => [link.href, link])).values());

		links;
	`, engine.ResultSelector)

	err := chromedp.Run(ctx, chromedp.Evaluate(script, &rawLinks))
	if err != nil {
		return nil, fmt.Errorf("failed to extract links: %w", err)
	}

	var results []string
	for _, link := range rawLinks {
		results = append(results, link["href"])
	}

	return results, nil
}

func (b *Browser) goToNextPage(ctx context.Context, engine SearchEngine) (bool, error) {
	var nodes []*cdp.Node
	err := chromedp.Run(ctx, chromedp.Nodes(engine.NextPageSelector, &nodes, chromedp.ByQuery))

	if err != nil || len(nodes) == 0 {
		b.logger.Info("Next page button not found",
			zap.String("selector", engine.NextPageSelector))
		return false, nil
	}

	var isDisabled bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			let btn = document.querySelector('%s');
			btn ? (btn.disabled || btn.classList.contains('disabled') || btn.getAttribute('aria-disabled') === 'true') : true
		`, engine.NextPageSelector), &isDisabled),
	)

	if err != nil {
		b.logger.Warn("Could not check if next button is disabled", zap.Error(err))
	}

	if isDisabled {
		b.logger.Info("Next page button is disabled")
		return false, nil
	}

	b.logger.Info("Clicking next page button", zap.String("selector", engine.NextPageSelector))

	err = chromedp.Run(ctx,
		chromedp.Click(engine.NextPageSelector, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.WaitReady("body"),
	)

	if err != nil {
		b.logger.Info("Failed to click next page button", zap.Error(err))
		return false, err
	}

	return true, nil
}

func (b *Browser) logDOMBeforeNextPage(ctx context.Context, engine SearchEngine) {
	var domHTML, currentURL string

	err := chromedp.Run(ctx,
		chromedp.Location(&currentURL),
		chromedp.OuterHTML("html", &domHTML),
	)

	if err != nil {
		b.logger.Warn("Failed to get DOM for logging", zap.Error(err))
		return
	}

	b.logger.Info("DOM state before next page",
		zap.Int("current_page", b.currentPage),
		zap.String("current_url", currentURL),
		zap.Int("dom_length", len(domHTML)),
		zap.String("dom", domHTML),
		zap.String("next_selector", engine.NextPageSelector))

	b.logger.Debug("DOM snippet before pagination",
		zap.Int("page", b.currentPage),
		zap.String("dom_snippet", domHTML[:200]+"..."))
}

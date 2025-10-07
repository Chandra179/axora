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
	SupportedEngines []SearchEngine
	ChromedpOptions  []chromedp.ExecAllocatorOption

	maxPages    int
	currentPage int
	pageDelay   time.Duration
}

func NewBrowser(logger *zap.Logger, proxyURL string) *Browser {
	return &Browser{
		logger: logger,
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
			chromedp.Flag("accept-language", "en-US,en;q=0.9"),
			chromedp.Flag("accept-encoding", "gzip, deflate, br"),
			chromedp.Flag("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("exclude-switches", "enable-automation"),
			chromedp.Flag("disable-extensions", ""),
			chromedp.ProxyServer(proxyURL),
		),
		maxPages:    50,
		currentPage: 0,
		pageDelay:   time.Second * 2,
	}
}

func (b *Browser) CollectUrls(ctx context.Context, query string, collectedUrls chan string) error {
	engine := b.SupportedEngines[1]

	taskCtx, cancel, err := b.setupBrowserContext(ctx, time.Minute*5)
	if err != nil {
		return fmt.Errorf("failed to setup browser context: %w", err)
	}
	defer cancel()

	searchURL := fmt.Sprintf(engine.URLTemplate, url.QueryEscape(query))
	if err := b.navigateToPage(taskCtx, searchURL, engine.Name); err != nil {
		return fmt.Errorf("failed to navigate to first page: %w", err)
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
			return err
		}

		urlCount, err := b.extractLinksFromCurrentPage(taskCtx, engine, collectedUrls)
		if err != nil {
			b.logger.Error("Failed to extract links from page",
				zap.Error(err),
				zap.Int("page", b.currentPage))
			return err
		}

		b.logger.Info("Collected URLs from page",
			zap.Int("page", b.currentPage),
			zap.Int("urls_this_page", urlCount),
		)

		if b.currentPage >= b.maxPages {
			b.logger.Info("Reached maximum pages", zap.Int("max_pages", b.maxPages))
			break
		}

		hasNext, err := b.goToNextPage(taskCtx, engine)
		if err != nil {
			b.logger.Error("Failed to navigate to next page",
				zap.Error(err),
				zap.Int("current_page", b.currentPage))
			return err
		}

		if !hasNext {
			b.logger.Info("No more pages available", zap.Int("final_page", b.currentPage))
			break
		}

		if b.pageDelay > 0 {
			time.Sleep(b.pageDelay)
		}
	}

	b.logger.Info("Collect Urls completed",
		zap.Int("total_pages", b.currentPage),
		zap.String("engine", engine.Name))

	return nil
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

	b.logger.Debug("Page state",
		zap.String("url", currentURL),
		zap.String("title", title),
		zap.String("ready_state", readyState),
		zap.Int("page", b.currentPage))

	if title == "Access Denied" || title == "Blocked" {
		return fmt.Errorf("page access blocked: %s", title)
	}

	return nil
}

// Optimized version that streams URLs directly to channel
func (b *Browser) extractLinksFromCurrentPage(ctx context.Context, engine SearchEngine, collectedUrls chan string) (int, error) {
	// Efficient script that only returns unique href strings
	script := fmt.Sprintf(`
		(function() {
			const resultsDiv = document.querySelector('%s');
			const anchors = resultsDiv ? 
				resultsDiv.querySelectorAll('a[href]') : 
				document.querySelectorAll('a[href]');
			
			const urls = new Set();
			
			for (const link of anchors) {
				const href = link.href;
				if (href && 
					href.startsWith('https') && 
					!href.startsWith('javascript:') &&
					link.textContent.trim().length > 0) {
					urls.add(href);
				}
			}
			
			return Array.from(urls);
		})();
	`, engine.ResultSelector)

	var urls []string
	err := chromedp.Run(ctx, chromedp.Evaluate(script, &urls))
	if err != nil {
		return 0, fmt.Errorf("failed to extract links: %w", err)
	}

	count := 0
	for _, href := range urls {
		select {
		case collectedUrls <- href:
			count++
		case <-ctx.Done():
			return count, ctx.Err()
		}
	}

	return count, nil
}

func (b *Browser) goToNextPage(ctx context.Context, engine SearchEngine) (bool, error) {
	var nodes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Nodes(engine.NextPageSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil || len(nodes) == 0 {
		return false, nil
	}

	err = chromedp.Run(ctx,
		chromedp.WaitVisible(engine.NextPageSelector, chromedp.ByQuery),
	)
	if err != nil {
		return false, nil
	}

	b.logger.Debug("Clicking next page button", zap.String("selector", engine.NextPageSelector))

	err = chromedp.Run(ctx,
		chromedp.Click(engine.NextPageSelector, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		return false, err
	}

	return true, nil
}

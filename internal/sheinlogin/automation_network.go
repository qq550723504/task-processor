package sheinlogin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

func buildResponseCaptureItem(channel, url string, status int, body string) map[string]any {
	item := map[string]any{
		"channel": channel,
		"url":     url,
		"status":  status,
	}
	if preview := summarizeNetworkPayloadBody(body); preview != "" {
		item["bodyPreview"] = preview
	}
	return item
}

func installPageNetworkCapture(page playwright.Page) error {
	if page == nil {
		return nil
	}
	debugEnabled := sheinLoginDebugEnabled()
	capture := getOrCreatePageNetworkCapture(page)
	if capture.markInstalled() {
		recordResponse := func(channel string, response playwright.Response) {
			if response == nil {
				return
			}
			url := strings.TrimSpace(response.URL())
			if !shouldCaptureAuthPayloadURL(url) {
				return
			}
			capture.add(buildResponseCaptureItem(channel, url, response.Status(), ""))
		}
		page.OnResponse(func(response playwright.Response) {
			recordResponse("playwright_page_response", response)
		})
		page.Context().OnResponse(func(response playwright.Response) {
			recordResponse("playwright_context_response", response)
		})
	}
	if debugEnabled {
		eventCapture := getOrCreatePageEventCapture(page)
		if eventCapture.markInstalled() {
			if cdpSession, err := page.Context().NewCDPSession(page); err == nil && cdpSession != nil {
				_, _ = cdpSession.Send("Runtime.enable", nil)
				_, _ = cdpSession.Send("Log.enable", nil)
				cdpSession.On("Runtime.exceptionThrown", func(params map[string]interface{}) {
					eventCapture.add(summarizeCDPRuntimeException(params))
				})
				cdpSession.On("Log.entryAdded", func(params map[string]interface{}) {
					eventCapture.add(summarizeCDPLogEntry(params))
				})
			}
			page.OnConsole(func(message playwright.ConsoleMessage) {
				if message == nil {
					return
				}
				eventCapture.add(map[string]any{
					"channel": "console_" + strings.TrimSpace(message.Type()),
					"text":    summarizeBodyText(message.Text(), 500),
				})
			})
			page.OnPageError(func(err error) {
				if err == nil {
					return
				}
				eventCapture.add(map[string]any{
					"channel": "page_error",
					"message": summarizeBodyText(err.Error(), 500),
				})
			})
			page.OnRequestFailed(func(request playwright.Request) {
				if request == nil {
					return
				}
				url := strings.TrimSpace(request.URL())
				if !shouldCaptureAuthPayloadURL(url) {
					return
				}
				eventCapture.add(map[string]any{
					"channel": "request_failed",
					"url":     url,
					"method":  strings.TrimSpace(request.Method()),
				})
			})
		}
	}
	script := fmt.Sprintf(`
(() => {
  if (window.__codexAuthPayloadCaptureInstalled) return;
  window.__codexAuthPayloadCaptureInstalled = true;
  const debugEnabled = %t;
  window.__codexAuthPayloads = [];
  if (debugEnabled) {
    window.__codexPageEvents = [];
  }
  const shouldCapture = (url) => {
    const lowered = String(url || '').toLowerCase();
    return lowered.includes('/sso/authenticate/login')
      || lowered.includes('/sso/authenticate/islogin')
      || lowered.includes('/sso/geetest/ajax.php')
      || lowered.includes('/sso/geetest/reset.php');
  };
  const summarizeText = (value) => String(value || '').replace(/\s+/g, ' ').trim().slice(0, 200);
  const describeElement = (el) => {
    if (!el || !el.tagName) return {};
    return {
      tag: String(el.tagName || ''),
      type: el.getAttribute ? el.getAttribute('type') : null,
      id: el.id || null,
      name: el.getAttribute ? el.getAttribute('name') : null,
      classes: el.className ? String(el.className).slice(0, 200) : null,
      text: summarizeText(el.innerText || el.textContent || ''),
    };
  };
  const pushPageEvent = (item) => {
    if (!debugEnabled) return;
    try {
      window.__codexPageEvents.push({
        ...item,
        capturedAt: Date.now(),
      });
      if (window.__codexPageEvents.length > 80) {
        window.__codexPageEvents = window.__codexPageEvents.slice(-80);
      }
    } catch (e) {}
  };
  const pushPayload = (item) => {
    try {
      window.__codexAuthPayloads.push({
        ...item,
        capturedAt: Date.now(),
      });
      if (window.__codexAuthPayloads.length > 30) {
        window.__codexAuthPayloads = window.__codexAuthPayloads.slice(-30);
      }
    } catch (e) {}
  };
  const origFetch = window.fetch;
  if (origFetch) {
    window.fetch = async function(...args) {
      let requestUrl = '';
      try {
        requestUrl = String((args[0] && args[0].url) || args[0] || '');
        if (shouldCapture(requestUrl)) {
          pushPageEvent({
            channel: 'fetch_start',
            url: requestUrl,
            method: args[1] && args[1].method ? String(args[1].method) : null,
          });
        }
      } catch (e) {}
      const response = await origFetch.apply(this, args);
      try {
        const url = response && response.url ? response.url : (args[0] && args[0].url) || args[0];
        if (shouldCapture(url)) {
          const cloned = response.clone();
          let body = '';
          try { body = await cloned.text(); } catch (e) {}
          pushPayload({
            channel: 'fetch',
            url: String(url || ''),
            status: response.status,
            bodyPreview: String(body || '').replace(/\s+/g, ' ').slice(0, 1000),
          });
          pushPageEvent({
            channel: 'fetch_end',
            url: String(url || ''),
            status: response.status,
          });
        }
      } catch (e) {}
      return response;
    };
  }
  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function(method, url, ...rest) {
    try {
      this.__codexCaptureUrl = url;
      this.__codexCaptureMethod = method;
    } catch (e) {}
    return origOpen.call(this, method, url, ...rest);
  };
  XMLHttpRequest.prototype.send = function(...args) {
    try {
      if (shouldCapture(this.__codexCaptureUrl)) {
        pushPageEvent({
          channel: 'xhr_start',
          url: String(this.__codexCaptureUrl || ''),
          method: this.__codexCaptureMethod ? String(this.__codexCaptureMethod) : null,
        });
      }
      this.addEventListener('loadend', function() {
        try {
          const url = this.__codexCaptureUrl || this.responseURL;
          if (!shouldCapture(url)) return;
          const body = typeof this.responseText === 'string' ? this.responseText : '';
          pushPayload({
            channel: 'xhr',
            url: String(url || ''),
            status: this.status,
            bodyPreview: String(body || '').replace(/\s+/g, ' ').slice(0, 1000),
          });
          pushPageEvent({
            channel: 'xhr_end',
            url: String(url || ''),
            status: this.status,
          });
        } catch (e) {}
      });
    } catch (e) {}
    return origSend.apply(this, args);
  };
  document.addEventListener('click', function(event) {
    try {
      const target = event.target && event.target.closest ? event.target.closest('button, a, input[type="submit"], [role="button"]') : null;
      if (!target) return;
      pushPageEvent({
        channel: 'dom_click',
        ...describeElement(target),
      });
    } catch (e) {}
  }, true);
  const recordSubmitEvent = (channel, event) => {
    try {
      const form = event.target;
      pushPageEvent({
        channel,
        action: form && form.getAttribute ? form.getAttribute('action') : null,
        method: form && form.getAttribute ? form.getAttribute('method') : null,
        defaultPrevented: !!event.defaultPrevented,
      });
    } catch (e) {}
  };
  document.addEventListener('submit', function(event) {
    recordSubmitEvent('form_submit_capture', event);
    queueMicrotask(() => recordSubmitEvent('form_submit_post_capture', event));
  }, true);
  document.addEventListener('submit', function(event) {
    recordSubmitEvent('form_submit_bubble', event);
    queueMicrotask(() => recordSubmitEvent('form_submit_post_bubble', event));
  }, false);
  document.addEventListener('invalid', function(event) {
    try {
      pushPageEvent({
        channel: 'form_invalid',
        validationMessage: event.target && typeof event.target.validationMessage === 'string' ? event.target.validationMessage : null,
        target: describeElement(event.target),
      });
    } catch (e) {}
  }, true);
  window.addEventListener('error', function(event) {
    try {
      const target = event && event.target && event.target !== window ? event.target : null;
      pushPageEvent({
        channel: 'window_error',
        message: event && event.message ? String(event.message) : null,
        filename: event && event.filename ? String(event.filename) : null,
        lineno: event && typeof event.lineno === 'number' ? event.lineno : null,
        colno: event && typeof event.colno === 'number' ? event.colno : null,
        stack: event && event.error && event.error.stack ? summarizeText(event.error.stack) : null,
        targetTag: target && target.tagName ? String(target.tagName) : null,
        targetSrc: target && target.src ? String(target.src) : null,
        targetHref: target && target.href ? String(target.href) : null,
      });
    } catch (e) {}
  }, true);
  window.addEventListener('unhandledrejection', function(event) {
    try {
      const reason = event && event.reason;
      pushPageEvent({
        channel: 'unhandled_rejection',
        reason: typeof reason === 'string' ? reason : summarizeText(reason && reason.message ? reason.message : JSON.stringify(reason)),
      });
    } catch (e) {}
  });
  const origRequestSubmit = HTMLFormElement.prototype.requestSubmit;
  if (origRequestSubmit) {
    HTMLFormElement.prototype.requestSubmit = function(...args) {
      try {
        pushPageEvent({
          channel: 'request_submit',
          action: this && this.getAttribute ? this.getAttribute('action') : null,
          method: this && this.getAttribute ? this.getAttribute('method') : null,
          submitter: describeElement(args[0]),
        });
      } catch (e) {}
      return origRequestSubmit.apply(this, args);
    };
  }
  const origSubmit = HTMLFormElement.prototype.submit;
  if (origSubmit) {
    HTMLFormElement.prototype.submit = function(...args) {
      try {
        pushPageEvent({
          channel: 'direct_submit',
          action: this && this.getAttribute ? this.getAttribute('action') : null,
          method: this && this.getAttribute ? this.getAttribute('method') : null,
        });
      } catch (e) {}
      return origSubmit.apply(this, args);
    };
  }
})();
`, debugEnabled)
	return page.Context().AddInitScript(playwright.Script{Content: playwright.String(script)})
}

func getCapturedNetworkPayloads(page playwright.Page) []map[string]any {
	if page == nil {
		return nil
	}
	var playwrightPayloads []map[string]any
	if capture := getPageNetworkCapture(page); capture != nil {
		playwrightPayloads = capture.snapshot()
	}
	return mergeNetworkPayloads(playwrightPayloads, getInjectedNetworkPayloads(page))
}

func mergeNetworkPayloads(playwrightPayloads, injectedPayloads []map[string]any) []map[string]any {
	if len(playwrightPayloads) == 0 && len(injectedPayloads) == 0 {
		return nil
	}
	merged := make([]map[string]any, 0, len(playwrightPayloads)+len(injectedPayloads))
	merged = append(merged, playwrightPayloads...)
	merged = append(merged, injectedPayloads...)
	return merged
}

func getInjectedNetworkPayloads(page playwright.Page) []map[string]any {
	return getInjectedMapItems(page, `() => Array.isArray(window.__codexAuthPayloads) ? window.__codexAuthPayloads.slice(-20) : []`)
}

func getInjectedPageEvents(page playwright.Page) []map[string]any {
	return getInjectedMapItems(page, `() => Array.isArray(window.__codexPageEvents) ? window.__codexPageEvents.slice(-40) : []`)
}

func getCapturedPageEvents(page playwright.Page) []map[string]any {
	var events []map[string]any
	if capture := getPageEventCapture(page); capture != nil {
		events = append(events, capture.snapshot()...)
	}
	events = append(events, getInjectedPageEvents(page)...)
	sortEventItems(events)
	return events
}

func getInjectedMapItems(page playwright.Page, expr string) []map[string]any {
	if page == nil {
		return nil
	}
	value, err := page.Evaluate(expr, nil)
	if err != nil || value == nil {
		return nil
	}
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if payload, ok := item.(map[string]interface{}); ok {
			result := make(map[string]any, len(payload))
			for k, v := range payload {
				result[k] = v
			}
			results = append(results, result)
		}
	}
	return results
}

func getOrCreatePageNetworkCapture(page playwright.Page) *pageNetworkCapture {
	if existing := getPageNetworkCapture(page); existing != nil {
		return existing
	}
	capture := &pageNetworkCapture{}
	actual, _ := pageNetworkCaptures.LoadOrStore(page, capture)
	stored, _ := actual.(*pageNetworkCapture)
	return stored
}

func getPageNetworkCapture(page playwright.Page) *pageNetworkCapture {
	if page == nil {
		return nil
	}
	actual, ok := pageNetworkCaptures.Load(page)
	if !ok {
		return nil
	}
	capture, _ := actual.(*pageNetworkCapture)
	return capture
}

func getOrCreatePageEventCapture(page playwright.Page) *pageEventCapture {
	if existing := getPageEventCapture(page); existing != nil {
		return existing
	}
	capture := &pageEventCapture{}
	actual, _ := pageEventCaptures.LoadOrStore(page, capture)
	stored, _ := actual.(*pageEventCapture)
	return stored
}

func getPageEventCapture(page playwright.Page) *pageEventCapture {
	if page == nil {
		return nil
	}
	actual, ok := pageEventCaptures.Load(page)
	if !ok {
		return nil
	}
	capture, _ := actual.(*pageEventCapture)
	return capture
}

func (c *pageNetworkCapture) markInstalled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.installed {
		return false
	}
	c.installed = true
	return true
}

func (c *pageEventCapture) markInstalled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.installed {
		return false
	}
	c.installed = true
	return true
}

func (c *pageNetworkCapture) add(item map[string]any) {
	if c == nil {
		return
	}
	normalized := make(map[string]any, len(item)+1)
	for k, v := range item {
		normalized[k] = v
	}
	normalized["capturedAt"] = time.Now().UnixMilli()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, normalized)
	if len(c.items) > 30 {
		c.items = append([]map[string]any(nil), c.items[len(c.items)-30:]...)
	}
}

func (c *pageEventCapture) add(item map[string]any) {
	if c == nil {
		return
	}
	normalized := make(map[string]any, len(item)+1)
	for k, v := range item {
		normalized[k] = v
	}
	normalized["capturedAt"] = time.Now().UnixMilli()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, normalized)
	if len(c.items) > 80 {
		c.items = append([]map[string]any(nil), c.items[len(c.items)-80:]...)
	}
}

func (c *pageNetworkCapture) snapshot() []map[string]any {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(c.items))
	for _, item := range c.items {
		copied := make(map[string]any, len(item))
		for k, v := range item {
			copied[k] = v
		}
		results = append(results, copied)
	}
	return results
}

func (c *pageEventCapture) snapshot() []map[string]any {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(c.items))
	for _, item := range c.items {
		copied := make(map[string]any, len(item))
		for k, v := range item {
			copied[k] = v
		}
		results = append(results, copied)
	}
	return results
}

func sortEventItems(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		return eventCapturedAt(items[i]) < eventCapturedAt(items[j])
	})
}

func eventCapturedAt(item map[string]any) int64 {
	switch value := item["capturedAt"].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	default:
		return 0
	}
}

func summarizeCDPRuntimeException(params map[string]interface{}) map[string]any {
	item := map[string]any{
		"channel": "cdp_runtime_exception",
	}
	details, _ := params["exceptionDetails"].(map[string]interface{})
	if len(details) == 0 {
		return item
	}
	if text := strings.TrimSpace(anyString(details["text"])); text != "" {
		item["text"] = summarizeBodyText(text, 500)
	}
	if url := strings.TrimSpace(anyString(details["url"])); url != "" {
		item["url"] = url
	}
	if line, ok := anyInt64(details["lineNumber"]); ok {
		item["lineNumber"] = line
	}
	if col, ok := anyInt64(details["columnNumber"]); ok {
		item["columnNumber"] = col
	}
	if exception, _ := details["exception"].(map[string]interface{}); len(exception) > 0 {
		if description := strings.TrimSpace(anyString(exception["description"])); description != "" {
			item["description"] = summarizeBodyText(description, 1000)
		}
		if value := strings.TrimSpace(anyString(exception["value"])); value != "" {
			item["value"] = summarizeBodyText(value, 500)
		}
	}
	if stack, _ := details["stackTrace"].(map[string]interface{}); len(stack) > 0 {
		if frames, ok := stack["callFrames"].([]interface{}); ok && len(frames) > 0 {
			summarized := make([]map[string]any, 0, minInt(len(frames), 5))
			for _, raw := range frames {
				frame, _ := raw.(map[string]interface{})
				if len(frame) == 0 {
					continue
				}
				summarized = append(summarized, map[string]any{
					"functionName": strings.TrimSpace(anyString(frame["functionName"])),
					"url":          strings.TrimSpace(anyString(frame["url"])),
					"lineNumber":   anyInt64Default(frame["lineNumber"]),
					"columnNumber": anyInt64Default(frame["columnNumber"]),
				})
				if len(summarized) >= 5 {
					break
				}
			}
			if len(summarized) > 0 {
				item["stack"] = summarized
			}
		}
	}
	return item
}

func summarizeCDPLogEntry(params map[string]interface{}) map[string]any {
	item := map[string]any{
		"channel": "cdp_log_entry",
	}
	entry, _ := params["entry"].(map[string]interface{})
	if len(entry) == 0 {
		return item
	}
	if source := strings.TrimSpace(anyString(entry["source"])); source != "" {
		item["source"] = source
	}
	if level := strings.TrimSpace(anyString(entry["level"])); level != "" {
		item["level"] = level
	}
	if text := strings.TrimSpace(anyString(entry["text"])); text != "" {
		item["text"] = summarizeBodyText(text, 1000)
	}
	if url := strings.TrimSpace(anyString(entry["url"])); url != "" {
		item["url"] = url
	}
	if line, ok := anyInt64(entry["lineNumber"]); ok {
		item["lineNumber"] = line
	}
	return item
}

func anyString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func anyInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		n, err := typed.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func anyInt64Default(value interface{}) int64 {
	if n, ok := anyInt64(value); ok {
		return n
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func shouldCaptureAuthPayloadURL(url string) bool {
	lowered := strings.ToLower(strings.TrimSpace(url))
	if lowered == "" {
		return false
	}
	return isSHEINLoginResponseURL(lowered) ||
		strings.Contains(lowered, "/sso/geetest/ajax.php") ||
		strings.Contains(lowered, "/sso/geetest/reset.php")
}

func isSHEINLoginResponseURL(url string) bool {
	lowered := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(lowered, "/sso/authenticate/login") ||
		strings.Contains(lowered, "/sso/authenticate/islogin")
}

func networkPayloadsConfirmSHEINLogin(payloads []map[string]any) bool {
	for i := len(payloads) - 1; i >= 0; i-- {
		payload := payloads[i]
		if !isSHEINLoginResponseURL(fmt.Sprint(payload["url"])) {
			continue
		}
		status, ok := anyInt64(payload["status"])
		if !ok || status < 200 || status >= 300 {
			continue
		}
		body := strings.TrimSpace(fmt.Sprint(payload["bodyPreview"]))
		if body == "" {
			body = strings.TrimSpace(fmt.Sprint(payload["body_preview"]))
		}
		if sheinLoginResponseSucceeded(body) {
			return true
		}
	}
	return false
}

func sheinLoginResponseSucceeded(body string) bool {
	var payload struct {
		Code any `json:"code"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &payload); err != nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(payload.Code)) == "0"
}

func summarizeNetworkPayloadBody(body string) string {
	return summarizeBodyText(body, 1000)
}

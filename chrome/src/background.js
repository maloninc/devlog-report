const DEFAULT_ENDPOINT = "http://127.0.0.1:8787/events";

let currentSpan = null;

function nowISO() {
  return new Date().toISOString();
}

function isTrackableURL(url) {
  return url.startsWith("http://") || url.startsWith("https://");
}

async function getEndpoint() {
  const stored = await chrome.storage.local.get("devlogEndpoint");
  return stored.devlogEndpoint || DEFAULT_ENDPOINT;
}

async function getActiveTab() {
  const tabs = await chrome.tabs.query({
    active: true,
    lastFocusedWindow: true,
  });
  if (!tabs || tabs.length === 0) {
    return null;
  }
  return tabs[0];
}

function buildEvent(span, endTs) {
  return {
    type: "browser_active_span",
    source: "chrome",
    event_id: crypto.randomUUID(),
    schema_version: 2,
    start_ts: span.startTs,
    end_ts: endTs,
    url: span.url,
    title: span.title || "",
  };
}

async function sendEvent(event) {
  const endpoint = await getEndpoint();
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 1000);
  try {
    await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(event),
      signal: controller.signal,
    });
  } catch {
    // Ignore network errors; this is best-effort.
  } finally {
    clearTimeout(timeoutId);
  }
}

async function endSpan(reason) {
  if (!currentSpan) {
    return;
  }
  const endTs = nowISO();
  const event = buildEvent(currentSpan, endTs);
  currentSpan = null;
  await sendEvent(event);
}

async function startSpanFromTab(tab) {
  if (!tab || !tab.url || !isTrackableURL(tab.url)) {
    return;
  }
  currentSpan = {
    tabId: tab.id,
    url: tab.url,
    title: tab.title || "",
    startTs: nowISO(),
  };
}

async function syncActiveSpan() {
  const tab = await getActiveTab();
  if (!tab || !tab.url || !isTrackableURL(tab.url)) {
    await endSpan("no-active-tab");
    return;
  }

  if (
    currentSpan &&
    currentSpan.tabId === tab.id &&
    currentSpan.url === tab.url
  ) {
    return;
  }

  await endSpan("tab-change");
  await startSpanFromTab(tab);
}

chrome.tabs.onActivated.addListener(() => {
  syncActiveSpan();
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (!tab.active) {
    return;
  }
  if (changeInfo.url) {
    syncActiveSpan();
  }
});

chrome.windows.onFocusChanged.addListener((windowId) => {
  if (windowId === chrome.windows.WINDOW_ID_NONE) {
    endSpan("window-blur");
    return;
  }
  syncActiveSpan();
});

chrome.idle.onStateChanged.addListener((state) => {
  if (state === "active") {
    syncActiveSpan();
    return;
  }
  endSpan("idle");
});

chrome.runtime.onStartup.addListener(() => {
  syncActiveSpan();
});

chrome.runtime.onInstalled.addListener(() => {
  syncActiveSpan();
});

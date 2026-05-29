const KEYSAT_URL = "http://127.0.0.1:7890/api/report/domain";

function reportDomain(url) {
  try {
    const parsed = new URL(url);
    // Only report real web domains, skip browser internal pages.
    if (parsed.protocol !== "https:" && parsed.protocol !== "http:") return;
    const domain = parsed.hostname;
    if (!domain || domain === "127.0.0.1" || domain === "localhost") return;
    fetch(KEYSAT_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ domain }),
    }).catch(() => {}); // silently fail if daemon not running
  } catch {}
}

// Report on tab activation
chrome.tabs.onActivated.addListener(async (activeInfo) => {
  const tab = await chrome.tabs.get(activeInfo.tabId);
  if (tab.url) reportDomain(tab.url);
});

// Report on tab URL change
chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.url && tab.active) {
    reportDomain(changeInfo.url);
  }
});

// Report current tab on startup
chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
  if (tabs[0]?.url) reportDomain(tabs[0].url);
});

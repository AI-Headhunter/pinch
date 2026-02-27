/** Derive the HTTP base URL from a WebSocket relay URL. */
export function relayBaseUrl(relayUrl: string): string {
	return relayUrl
		.replace(/^wss:\/\//, "https://")
		.replace(/^ws:\/\//, "http://")
		.replace(/\/ws$/, "");
}

class OrraSDK {
	#apiUrl;
	#apiKey;
	#ws;
	#taskHandler;
	
	constructor(apiUrl, apiKey) {
		this.#apiUrl = apiUrl;
		this.#apiKey = apiKey;
		this.#ws = null;
		this.#taskHandler = null;
	}
	
	async registerService(serviceName, opts = {
		description: undefined,
		schema: undefined,
		version: '',
	}) {
		const response = await fetch(`${this.#apiUrl}/register/service`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				name: serviceName,
				description: opts?.description,
				schema: opts?.schema
			}),
		});
		
		if (!response.ok) {
			throw new Error(`Failed to register service: ${response.statusText}`);
		}
		this.#setupWebSocket(serviceName);
		return this
	}
	
	#setupWebSocket(serviceName) {
		this.ws = new WebSocket(`${this.#apiUrl.replace('http', 'ws')}/ws?service=${serviceName}`);
		
		this.ws.onmessage = async (event) => {
			if (!this.#taskHandler) return;
			
			const task = JSON.parse(event.data);
			try {
				const result = await this.#taskHandler(task);
				this.ws.send(JSON.stringify({ taskId: task.id, result }));
			} catch (error) {
				console.error('Error handling task:', error);
				this.ws.send(JSON.stringify({ taskId: task.id, error: error.message }));
			}
		};
		
		this.ws.onerror = (error) => {
			console.error('WebSocket error:', error);
		};
		
		this.ws.onclose = () => {
			console.log('WebSocket closed. Attempting to reconnect...');
			setTimeout(() => this.#setupWebSocket(serviceName), 5000);
		};
	}
	
	startHandler(handler) {
		this.#taskHandler = handler;
	}
	
	close() {
		if (this.ws) {
			this.ws.close();
		}
	}
}

export function createClient(config = {
	orraUrl: undefined,
	orraKey: undefined
}) {
	if (!config?.orraUrl || config?.orraKey) {
		throw "Cannot create an SDK client: ensure both a valid Orra URL and Orra API Key have been provided."
	}
	return new OrraSDK(config?.orraUrl, config?.orraKey);
}

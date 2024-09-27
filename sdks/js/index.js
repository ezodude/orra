import WebSocket from 'ws';

class OrraSDK {
	#apiUrl;
	#apiKey;
	#ws;
	#taskHandler;
	serviceId;
	#reconnectAttempts = 0;
	#maxReconnectAttempts = 5;
	#reconnectInterval = 5000; // 5 seconds
	
	constructor(apiUrl, apiKey) {
		this.#apiUrl = apiUrl;
		this.#apiKey = apiKey;
		this.#ws = null;
		this.#taskHandler = null;
		this.serviceId = null;
	}
	
	async #registerServiceOrAgent(name, kind, opts = {
		description: undefined,
		schema: undefined,
		version: '',
	}) {
		const response = await fetch(`${this.#apiUrl}/register/${kind}`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				'Authorization': `Bearer ${this.#apiKey}`
			},
			body: JSON.stringify({
				name: name,
				description: opts?.description,
				schema: opts?.schema
			}),
		});
		
		if (!response.ok) {
			const resText = await response.text()
			throw new Error(`Failed to register ${kind} because of ${response.statusText}: ${resText}`);
		}
		
		const data = await response.json();
		this.serviceId = data.id;
		
		if (!this.serviceId) {
			throw new Error(`${kind} ID was not received after registration`);
		}
		
		this.#setupWebSocket(this.serviceId);
		return this;
	}
	
	async registerService(name, opts = {
		description: undefined,
		schema: undefined,
		version: '',
	}) {
		return this.#registerServiceOrAgent(name, "service", opts);
	}
	
	async registerAgent(name, opts = {
		description: undefined,
		schema: undefined,
		version: '',
	}) {
		return this.#registerServiceOrAgent(name, "agent", opts);
	}
	
	#setupWebSocket(serviceId) {
		this.#ws = new WebSocket(`${this.#apiUrl.replace('http', 'ws')}/ws?serviceId=${serviceId}`);
		
		this.#ws.onmessage = async (event) => {
			if (!this.#taskHandler) return;
			
			const task = JSON.parse(event.data);
			try {
				const result = await this.#taskHandler(task);
				this.#ws.send(JSON.stringify({ taskId: task.id, result }));
			} catch (error) {
				console.error('Error handling task:', error);
				this.#ws.send(JSON.stringify({ taskId: task.id, error: error.message }));
			}
		};
		
		this.#ws.onerror = (error) => {
			console.error('WebSocket error:', error);
		};
		
		this.#ws.onclose = () => {
			console.log('WebSocket closed. Attempting to reconnect...');
			this.#reconnect();
		};
	}
	
	async #reconnect() {
		if (this.#reconnectAttempts >= this.#maxReconnectAttempts) {
			console.error('Max reconnection attempts reached. Please check your connection.');
			return;
		}
		
		this.#reconnectAttempts++;
		
		try {
			await this.#setupWebSocket(this.serviceId);
			this.#reconnectAttempts = 0;
		} catch (error) {
			console.error('Reconnection error:', error);
			setTimeout(() => this.#reconnect(), this.#reconnectInterval);
		}
	}
	
	startHandler(handler) {
		this.#taskHandler = handler;
	}
	
	close() {
		if (this.#ws) {
			this.#ws.close();
		}
	}
}

export function createClient(config = {
	orraUrl: undefined,
	orraKey: undefined
}) {
	if (!config?.orraUrl || !config?.orraKey) {
		throw "Cannot create an SDK client: ensure both a valid Orra URL and Orra API Key have been provided.";
	}
	return new OrraSDK(config?.orraUrl, config?.orraKey);
}

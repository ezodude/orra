class OrraSDK {
	#apiUrl;
	#apiKey;
	#ws;
	#taskHandler;
	serviceId;
	
	constructor(apiUrl, apiKey) {
		this.#apiUrl = apiUrl;
		this.#apiKey = apiKey;
		this.#ws = null;
		this.#taskHandler = null;
		this.serviceId = null;
	}
	
	async registerService(serviceName, opts = {
		description: undefined,
		schema: undefined,
		version: '',
	}) {
		const response = await fetch(`${this.#apiUrl}/register/service`, {
			method: 'POST', headers: {
				'Content-Type': 'application/json', 'Authorization': `Bearer ${this.#apiKey}` // Make sure to include the API key
			}, body: JSON.stringify({
				name: serviceName,
				description: opts?.description,
				schema: opts?.schema
			}),
		});
		
		if (!response.ok) {
			throw new Error(`Failed to register service: ${response.statusText}`);
		}
		
		const data = await response.json();
		
		this.serviceId = data.id;
		if (this.serviceId) {
			throw new Error('Service ID was not received after registering the service');
		}
		
		this.#setupWebSocket(this.serviceId);
		return this
	}
	
	#setupWebSocket(serviceId) {
		this.ws = new WebSocket(`${this.#apiUrl.replace('http', 'ws')}/ws?serviceId=${serviceId}`);
		
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
			setTimeout(() => this.#setupWebSocket(serviceId), 5000);
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
	orraUrl: undefined, orraKey: undefined
}) {
	if (!config?.orraUrl || config?.orraKey) {
		throw "Cannot create an SDK client: ensure both a valid Orra URL and Orra API Key have been provided."
	}
	return new OrraSDK(config?.orraUrl, config?.orraKey);
}

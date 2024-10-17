import WebSocket from 'ws';

class OrraSDK {
	#apiUrl;
	#apiKey;
	#ws;
	#taskHandler;
	serviceId;
	#reconnectAttempts = 0;
	#maxReconnectAttempts = 10;
	#reconnectInterval = 1000; // 1 seconds
	#maxReconnectInterval = 30000 // Max 30 seconds
	#messageQueue = [];
	#isConnected = false;
	
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
		
		this.#connect();
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
	
	#connect() {
		const wsUrl = this.#apiUrl.replace('http', 'ws');
		this.#ws = new WebSocket(`${wsUrl}/ws?serviceId=${this.serviceId}&apiKey=${this.#apiKey}`);
		
		this.#ws.onopen = () => {
			this.#isConnected = true;
			this.#reconnectAttempts = 0;
			this.#reconnectInterval = 1000;
			this.#sendQueuedMessages();
		};
		
		this.#ws.onmessage = async (event) => {
			const data = event.data;
			if (data === 'ping') {
				this.#ws.send(JSON.stringify({ type: 'pong' }));
			} else {
				// Handle other messages
				if (!this.#taskHandler) return;
				
				const task = JSON.parse(event.data);
				const { id: taskId, executionId } = task;
				
				try {
					const result = await this.#taskHandler(task);
					this.#sendTaskResult(taskId, executionId, result);
				} catch (error) {
					console.error('Error handling task:', error);
					this.#sendTaskResult(taskId, executionId, null, error.message);
				}
			}
		};
		
		this.#ws.onclose = async (event) => {
			this.#isConnected = false;
			if (event.wasClean) {
				console.log(`WebSocket closed cleanly, code=${event.code}, reason=${event.reason}`);
			} else {
				console.log('WebSocket connection died');
			}
			await this.#reconnect();
		};
		
		this.#ws.onerror = (error) => {
			console.error('WebSocket error:', error);
		};
	}
	
	async #reconnect() {
		if (this.#reconnectAttempts >= this.#maxReconnectAttempts) {
			console.log('Max reconnection attempts reached. Giving up.');
			return;
		}
		
		this.#reconnectAttempts++;
		const delay = Math.min(this.#reconnectInterval * Math.pow(2, this.#reconnectAttempts), this.#maxReconnectInterval);
		
		console.log(`Attempting to reconnect in ${delay}ms...`);
		
		setTimeout(async () => {
			console.log('Reconnecting...');
			await this.#connect();
		}, delay);
	}
	
	#sendTaskResult(taskId, executionId, result, error = null) {
		const message = {
			type: 'task_result',
			taskId,
			executionId,
			result,
			error
		};
		this.#sendMessage(message);
	}
	
	#sendMessage(message) {
		if (this.#isConnected && this.#ws.readyState === WebSocket.OPEN) {
			this.#ws.send(JSON.stringify(message));
		} else {
			console.log('WebSocket is not open. Queueing message.');
			this.#messageQueue.push(message);
		}
	}
	
	#sendQueuedMessages() {
		while (this.#messageQueue.length > 0 && this.#isConnected && this.#ws.readyState === WebSocket.OPEN) {
			const message = this.#messageQueue.shift();
			this.#ws.send(JSON.stringify(message));
			console.log('Sent queued message:', message);
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

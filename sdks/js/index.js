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
	#messageId = 0;
	#pendingMessages = new Map();
	
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
		
		this.#ws.onmessage = (event) => {
			const data = event.data;
			
			if (data === 'ping') {
				this.#handlePing();
				return;
			}
			
			let parsedData;
			try {
				parsedData = JSON.parse(data);
			} catch (error) {
				console.error('Failed to parse WebSocket message:', error);
				return;
			}
			
			switch (parsedData.type) {
				case 'ACK':
					this.#handleAcknowledgment(parsedData);
					break;
				case 'task':
					this.#handleTask(parsedData);
					break;
				default:
					console.warn('Received unknown message type:', parsedData.type);
			}
		};
		
		this.#ws.onclose = (event) => {
			this.#isConnected = false;
			for (const message of this.#pendingMessages.values()) {
				this.#messageQueue.push(message);
			}
			this.#pendingMessages.clear();
			
			if (event.wasClean) {
				console.log(`WebSocket closed cleanly, code=${event.code}, reason=${event.reason}`);
			} else {
				console.log('WebSocket connection died');
			}
			this.#reconnect();
		};
		
		this.#ws.onerror = (error) => {
			console.error('WebSocket error:', error);
		};
	}
	
	#handlePing() {
		console.log("Received PING");
		this.#sendPong();
		console.log("Sent PONG");
	}
	
	#sendPong() {
		if (this.#isConnected && this.#ws.readyState === WebSocket.OPEN) {
			this.#ws.send(JSON.stringify({ id: "pong", payload: { type: 'pong' } }));
		}
	}
	
	#handleAcknowledgment(data) {
		console.log("Acknowledged sent message", data.id);
		this.#pendingMessages.delete(data.id);
	}
	
	#handleTask(task) {
		if (!this.#taskHandler) {
			console.warn('Received task but no task handler is set');
			return;
		}
		
		const { id: taskId, executionId } = task;
		
		Promise.resolve(this.#taskHandler(task))
			.then((result) => {
				console.log(`Handled task:`, task);
				this.#sendTaskResult(taskId, executionId, result);
			})
			.catch((error) => {
				console.error('Error handling task:', error);
				this.#sendTaskResult(taskId, executionId, null, error.message);
			});
	}
	
	
	#reconnect() {
		if (this.#reconnectAttempts >= this.#maxReconnectAttempts) {
			console.log('Max reconnection attempts reached. Giving up.');
			return;
		}
		
		this.#reconnectAttempts++;
		const delay = Math.min(this.#reconnectInterval * Math.pow(2, this.#reconnectAttempts), this.#maxReconnectInterval);
		
		console.log(`Attempting to reconnect in ${delay}ms...`);
		
		setTimeout(() => {
			console.log('Reconnecting...');
			this.#connect();
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
		this.#messageId++
		const id = `message_${this.#messageId}_${message.executionId}`;
		const wrappedMessage = { id, payload: message };
		
		console.log("About to send message:", id);
		if (this.#isConnected && this.#ws.readyState === WebSocket.OPEN) {
			
			try {
				this.#ws.send(JSON.stringify(wrappedMessage));
				console.log("Sending message:", id);
				this.#pendingMessages.set(id, message);
				// Set a timeout to move message back to queue if no ACK received
				setTimeout(() => this.#handleMessageTimeout(id), 5000);
				
			} catch (e) {
				console.log('Message failed to send. Queueing message:', e.message);
				this.#messageQueue.push(message);
			}
			
		} else {
			console.log('WebSocket is not open. Queueing message.');
			this.#messageQueue.push(message);
		}
	}
	
	#handleMessageTimeout(id) {
		if (this.#pendingMessages.has(id)) {
			const message = this.#pendingMessages.get(id);
			this.#pendingMessages.delete(id);
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

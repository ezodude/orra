const { createServer } = require('http');
const { Server } = require("socket.io");
const next = require('next');
const axios = require('axios');
const dotenv = require('dotenv');
const { parse } = require('url');

// Load environment variables
dotenv.config({ path: '.env.local' });

const ORRA_URL = process?.env?.ORRA_URL;
const ORRA_API_KEY = process?.env?.ORRA_API_KEY;
const dev = process.env.NODE_ENV !== 'production';

const app = next({ dev });
const handle = app.getRequestHandler();

const DEFAULT_ACTION = 'I am interested in this product when can I receive it?'
const DEFAULT_PRODUCT_DESCRIPTION = 'Peanuts collectible Swatch with red straps.'

function splitAndTrim(sentence) {
	if (!sentence || sentence.length < 1) {
		return []
	}
	
	// Regular expression to match punctuation
	const punctuationRegex = /[.!?:]/;
	
	// Split the sentence using the punctuation regex
	const parts = sentence.split(punctuationRegex);
	
	// Trim each part and filter out empty strings
	return parts
		.map(part => part.trim())
		.filter(part => part.length > 0);
}

app.prepare().then(() => {
	const server = createServer((req, res) => {
		const parsedUrl = parse(req.url, true);
		handle(req, res, parsedUrl);
	});
	
	const io = new Server(server);
	
	// Store the io instance on the global object
	global.io = io;
	
	io.on('connection', (socket) => {
		console.log('A client connected');
		
		socket.on('chat message', async (msg) => {
			console.log('Message received:', msg);
			
			// Broadcast the message to all connected clients
			io.emit('chat message', msg);
			
			// Make a POST request to an external system
			try {
				let action = DEFAULT_ACTION;
				let productDescription = DEFAULT_PRODUCT_DESCRIPTION;
				
				const parts = splitAndTrim(msg.content);
				
				if (parts.length > 0) {
					action = parts[0];
				}
				
				if (parts.length > 1) {
					productDescription = parts[1];
				}
				
				const payload = {
					action: {
						type: 'ecommerce',
						content: action
					},
					data: [
						{
							field: "customerId",
							value: msg?.customerId,
						},
						{
							field: "productDescription",
							value: productDescription,
						}
					]
				};
				const response = await axios.post(ORRA_URL, payload,
					{
						headers: {
							'Authorization': `Bearer ${ORRA_API_KEY}`,
							'Content-Type': 'application/json'
						}
					});
				console.log('External API response:', response.data);
				io.emit('orra_plan', response.data?.plan);
			} catch (error) {
				console.error('Error posting to external API:', error);
			}
		});
		
		socket.on('disconnect', () => {
			console.log('A client disconnected');
		});
	});
	
	server.listen(3000, (err) => {
		if (err) throw err;
		console.log('> Ready on http://localhost:3000');
	});
});

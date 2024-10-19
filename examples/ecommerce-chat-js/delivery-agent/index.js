import { createClient } from '@orra/sdk';
import { runAgent } from "./agent.js";
import dotenv from 'dotenv';

// Load environment variables
dotenv.config();

// Configuration from environment variables
const sdkConfig = {
	orraUrl: process.env.ORRA_URL,
	orraKey: process.env.ORRA_API_KEY
};

// Validate environment variables
if (!sdkConfig.orraUrl || !sdkConfig.orraKey) {
	console.error('Error: ORRA_URL, ORRA_API_KEY and MISTRAL_API_KEY must be set in the environment variables');
	process.exit(1);
}

// Create the Orra SDK client
const orra = createClient(sdkConfig);

// Service details
const agentName = 'DeliveryAgent';
const agentDescription = 'An agent that helps customers with estimated delivery dates for online shopping.';
const agentSchema = {
	input: {
		type: 'object',
		properties: {
			customerId: { type: 'string' },
			customerName: { type: 'string' },
			customerAddress: { type: 'string' },
			productDescription: { type: 'string' },
			productAvailability: { type: 'string' },
			warehouseAddress: { type: 'string' },
		},
		required: [ 'customerId', 'customerAddress', 'warehouseAddress', 'productAvailability' ]
	},
	output: {
		type: 'object',
		properties: {
			response: { type: 'string' }
		},
		required: [ 'response' ]
	}
};

// Task handler function
async function handleTask(task) {
	console.log('Received task:', task);
	
	// Extract the message from the task input
	const {
		customerId,
		customerName,
		customerAddress,
		productDescription,
		productAvailability,
		warehouseAddress
	} = task.input;
	
	console.log('Running agent:', agentName)
	const response = await runAgent({
		customerId,
		customerName,
		customerAddress,
		productDescription,
		productAvailability,
		warehouseAddress,
	})
	console.log('Agent task response:', response)
	
	return { response: response };
}

// Main function to set up and run the service
async function main() {
	try {
		// Register the service
		await orra.registerAgent(agentName, {
			description: agentDescription,
			schema: agentSchema
		});
		console.log(agentName, ' registered successfully');
		
		// Start the task handler
		orra.startHandler(handleTask);
		console.log('Task handler started');
		
	} catch (error) {
		console.error('Error setting up the agent:', error);
	}
}

// Run the main function
main();

// Handle graceful shutdown
process.on('SIGINT', () => {
	console.log('Shutting down...');
	orra.close();
	process.exit();
});

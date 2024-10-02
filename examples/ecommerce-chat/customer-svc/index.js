import { createClient } from '@orra/sdk';
import dotenv from 'dotenv';

// Load environment variables
dotenv.config();

// Configuration from environment variables
const config = {
	orraUrl: process.env.ORRA_URL,
	orraKey: process.env.ORRA_KEY
};

// Validate environment variables
if (!config.orraUrl || !config.orraKey) {
	console.error('Error: ORRA_URL and ORRA_KEY must be set in the environment variables');
	process.exit(1);
}

// Create the Orra SDK client
const orra = createClient(config);

// Service details
const serviceName = 'CustomerService';
const serviceDescription = 'A service that retrieves and manages customer information.'
const serviceSchema = {
	input: {
		type: 'object',
		properties: {
			customerId: { type: 'string' }
		},
		required: ['customerId']
	},
	output: {
		type: 'object',
		properties: {
			customerId: { type: 'string' },
			customerName: { type: 'string' },
			customerAddress: { type: 'string' },
		},
		required: ['customerId', 'customerName', 'customerAddress'],
	}
};

// Task handler function
async function handleTask(task) {
	console.log('Received task:', task);
	
	// Extract the message from the task input
	const customerId  = task.input;
	
	// Send back customer data
	const result = {
		customerId: customerId,
		customerName: "Clark Kent",
		customerAddress: "1a Goldsmiths Row, London E2 8QA"
	};
	
	console.log('Sending result:', result);
	return result;
}

// Main function to set up and run the service
async function main() {
	try {
		// Register the service
		await orra.registerService(serviceName, {
			description: serviceDescription,
			schema: serviceSchema
		});
		console.log(`${serviceName} service registered successfully`);
		
		// Start the task handler
		orra.startHandler(handleTask);
		console.log('Task handler started');
		
	} catch (error) {
		console.error('Error setting up the service:', error);
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

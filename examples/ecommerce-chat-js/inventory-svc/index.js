import { createClient } from '@orra/sdk';
import dotenv from 'dotenv';

// Load environment variables
dotenv.config();

// Validate environment variables
if (!process.env.ORRA_URL || !process.env.ORRA_API_KEY) {
	console.error('Error: ORRA_URL and ORRA_API_KEY must be set in the environment variables');
	process.exit(1);
}

// Create the Orra SDK client
const orra = createClient({
	orraUrl: process.env.ORRA_URL,
	orraKey: process.env.ORRA_API_KEY,
});

// Service details
const serviceName = 'InventoryService';
const serviceDescription = 'An inventory service that manages and tracks the availability of ecommerce products. ' +
	'Including, updating inventory in real-time as orders are placed';
const serviceSchema = {
	input: {
		type: 'object',
		properties: {
			productDescription: { type: 'string' }
		},
		required: ['productDescription']
	},
	output: {
		type: 'object',
		properties: {
			productId: { type: 'string' },
			productDescription: { type: 'string' },
			productAvailability: { type: 'string' },
			warehouseAddress: { type: 'string' },
		},
		required: ['productId', 'productDescription', 'productAvailability']
	}
};

// Task handler function
async function handleTask(task) {
	console.log('Received task:', task);
	
	// Extract the productDescription from the task input
	const { productDescription }  = task.input;
	
	// Send back product data
	const result = {
		productId: '697d1744-88dd-4139-beeb-b307dfb1a2f9',
		productDescription: productDescription,
		productAvailability: 'AVAILABLE',
		warehouseAddress: 'Unit 1 Cairnrobin Way, Portlethen, Aberdeen AB12 4NJ'
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

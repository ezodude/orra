'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';
import io from 'socket.io-client';

const WebSocketContext = createContext({ socket: null, isConnected: false });

export function WebSocketProvider({ children }) {
	const [socket, setSocket] = useState(null);
	const [isConnected, setIsConnected] = useState(false);
	
	useEffect(() => {
		const newSocket = io();
		
		newSocket.on('connect', () => {
			setIsConnected(true);
		});
		
		newSocket.on('disconnect', () => {
			setIsConnected(false);
		});
		
		setSocket(newSocket);
		
		return () => {
			newSocket.close();
		};
	}, []);
	
	return (
		<WebSocketContext.Provider value={{ socket, isConnected }}>
			{children}
		</WebSocketContext.Provider>
	);
}

export const useWebSocket = () => useContext(WebSocketContext);

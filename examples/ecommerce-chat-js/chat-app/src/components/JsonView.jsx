import React, { useState } from 'react';

const JsonView = ({ data }) => {
	const [expanded, setExpanded] = useState({});
	
	const toggleExpand = (key) => {
		setExpanded(prev => ({ ...prev, [key]: !prev[key] }));
	};
	
	const renderValue = (value, key, path) => {
		if (typeof value === 'object' && value !== null) {
			return renderObject(value, key, path);
		}
		return <span className="json-value">{JSON.stringify(value)}</span>;
	};
	
	const renderObject = (obj, key, path = '') => {
		obj = typeof obj === 'string' ? JSON.parse(obj) : obj;
		const currentPath = path ? `${path}.${key}` : key;
		const isExpanded = expanded[currentPath];
		const isArray = Array.isArray(obj);
		const isEmpty = Object.keys(obj).length === 0;
		
		return (
			<div key={currentPath} className="json-object">
        <span
	        className="json-toggle"
	        onClick={() => toggleExpand(currentPath)}
        >
          {isExpanded ? '▼' : '▶'} {key}:
	        {isArray ? '[' : '{'}
        </span>
				{isExpanded && !isEmpty ? (
					<div className="json-nested">
						{Object.entries(obj).map(([k, v]) => (
							<div key={k} className="json-pair">
								{renderValue(v, k, currentPath)}
							</div>
						))}
					</div>
				) : null}
				{isExpanded ? (
					<span>{isArray ? ']' : '}'}</span>
				) : (
					<span> {isEmpty ? (isArray ? '[]' : '{}') : '...'}</span>
				)}
			</div>
		);
	};
	
	return (
		<div className="json-view">
			{renderObject(data, 'root')}
		</div>
	);
};

export default JsonView;

import React from 'react';

const services = [
  { name: 'user-service', team: 'backend' },
  { name: 'payment-service', team: 'payments' },
  { name: 'notification-service', team: 'platform' },
  { name: 'analytics-service', team: 'data' },
];

export default function ServiceExplorer() {
  return (
    <div className="bg-white rounded shadow p-4">
      <h2 className="text-lg font-bold mb-2">Services</h2>
      <ul className="space-y-1">
        {services.map(s => (
          <li key={s.name} className="flex justify-between items-center">
            <span>{s.name}</span>
            <span className="text-xs text-gray-500">{s.team}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

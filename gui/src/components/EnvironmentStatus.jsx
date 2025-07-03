import React, { useState, useEffect } from 'react';

function EnvironmentStatus() {
  // Mock environment data - in real app this would come from API
  const [envData] = useState({
    processingPods: {
      total: 8,
      active: 6,
      idle: 2,
      failed: 0
    },
    resources: {
      cpuUsage: 73,
      memoryUsage: 68,
      storageUsage: 45
    },
    services: {
      starlight: 'healthy',
      rabbitmq: 'healthy',
      postgres: 'healthy',
      s3bucket: 'healthy',
      apiGateway: 'warning'
    },
    cluster: {
      nodes: 3,
      region: 'eu-west-1',
      uptime: '7d 14h 23m'
    },
    processing: {
      activeJobs: 4,
      queuedJobs: 12,
      completedToday: 47,
      avgProcessingTime: '23m 14s'
    }
  });

  const getServiceStatusColor = (status) => {
    switch (status) {
      case 'healthy': return '#68D391';
      case 'warning': return '#F6AD55';
      case 'error': return '#FC8181';
      case 'offline': return '#A0AEC0';
      default: return '#A0AEC0';
    }
  };

  const getServiceStatusIcon = (status) => {
    switch (status) {
      case 'healthy': return '●';
      case 'warning': return '▲';
      case 'error': return '✕';
      case 'offline': return '○';
      default: return '○';
    }
  };

  const getUsageColor = (percentage) => {
    if (percentage >= 90) return '#FC8181';
    if (percentage >= 75) return '#F6AD55';
    if (percentage >= 50) return '#4FD1C5';
    return '#68D391';
  };

  return (
    <div className="environment-status">
      <div className="env-header">
        <h3>Environment Status</h3>
        <div className="env-cluster-info">
          <span className="cluster-detail">
            <span className="cluster-label">Cluster:</span>
            <span className="cluster-value">{envData.cluster.nodes} nodes • {envData.cluster.region}</span>
          </span>
          <span className="cluster-detail">
            <span className="cluster-label">Uptime:</span>
            <span className="cluster-value">{envData.cluster.uptime}</span>
          </span>
        </div>
      </div>

      <div className="env-content">
        {/* Processing Pods Section */}
        <div className="env-section">
          <div className="section-header">
            <h4>Processing Pods</h4>
            <div className="section-summary">{envData.processingPods.active}/{envData.processingPods.total} active</div>
          </div>
          <div className="pods-grid">
            <div className="pod-stat">
              <div className="pod-count active">{envData.processingPods.active}</div>
              <div className="pod-label">Active</div>
            </div>
            <div className="pod-stat">
              <div className="pod-count idle">{envData.processingPods.idle}</div>
              <div className="pod-label">Idle</div>
            </div>
            <div className="pod-stat">
              <div className="pod-count failed">{envData.processingPods.failed}</div>
              <div className="pod-label">Failed</div>
            </div>
          </div>
        </div>

        {/* Resource Usage Section */}
        <div className="env-section">
          <div className="section-header">
            <h4>Resource Usage</h4>
          </div>
          <div className="resources-grid">
            <div className="resource-item">
              <div className="resource-label">CPU</div>
              <div className="resource-bar">
                <div 
                  className="resource-fill"
                  style={{ 
                    width: `${envData.resources.cpuUsage}%`,
                    backgroundColor: getUsageColor(envData.resources.cpuUsage)
                  }}
                ></div>
              </div>
              <div className="resource-value">{envData.resources.cpuUsage}%</div>
            </div>
            <div className="resource-item">
              <div className="resource-label">Memory</div>
              <div className="resource-bar">
                <div 
                  className="resource-fill"
                  style={{ 
                    width: `${envData.resources.memoryUsage}%`,
                    backgroundColor: getUsageColor(envData.resources.memoryUsage)
                  }}
                ></div>
              </div>
              <div className="resource-value">{envData.resources.memoryUsage}%</div>
            </div>
            <div className="resource-item">
              <div className="resource-label">Storage</div>
              <div className="resource-bar">
                <div 
                  className="resource-fill"
                  style={{ 
                    width: `${envData.resources.storageUsage}%`,
                    backgroundColor: getUsageColor(envData.resources.storageUsage)
                  }}
                ></div>
              </div>
              <div className="resource-value">{envData.resources.storageUsage}%</div>
            </div>
          </div>
        </div>

        {/* Services Health Section */}
        <div className="env-section">
          <div className="section-header">
            <h4>Services</h4>
          </div>
          <div className="services-grid">
            {Object.entries(envData.services).map(([service, status]) => (
              <div key={service} className="service-item">
                <span 
                  className="service-status-icon"
                  style={{ color: getServiceStatusColor(status) }}
                >
                  {getServiceStatusIcon(status)}
                </span>
                <span className="service-name">{service}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Processing Stats Section */}
        <div className="env-section">
          <div className="section-header">
            <h4>Processing Stats</h4>
          </div>
          <div className="stats-grid">
            <div className="stat-item">
              <div className="stat-value">{envData.processing.activeJobs}</div>
              <div className="stat-label">Active Jobs</div>
            </div>
            <div className="stat-item">
              <div className="stat-value">{envData.processing.queuedJobs}</div>
              <div className="stat-label">Queued</div>
            </div>
            <div className="stat-item">
              <div className="stat-value">{envData.processing.completedToday}</div>
              <div className="stat-label">Completed Today</div>
            </div>
            <div className="stat-item">
              <div className="stat-value">{envData.processing.avgProcessingTime}</div>
              <div className="stat-label">Avg Time</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default EnvironmentStatus; 
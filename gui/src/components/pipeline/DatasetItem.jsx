import React from 'react';
import PropTypes from 'prop-types';
import { isProcessingComplete } from '../../utils/processing/processingUtils';

const DatasetItem = ({ 
  dataset, 
  isSelected, 
  onSelect, 
  onStartProcessing, 
  onDelete,
  inputFiles,
  outputFiles,
  processorType 
}) => {
  const processingComplete = isProcessingComplete(
    isSelected ? dataset.id : null,
    dataset.name, 
    inputFiles, 
    outputFiles, 
    processorType
  );

  return (
    <div className={`dataset-item-container ${isSelected ? 'active' : ''}`}>
      <button
        className="dataset-item"
        onClick={() => onSelect(dataset.id)}
      >
        <div className="dataset-info">
          <div className="dataset-name">{dataset.name}</div>
        </div>
      </button>
      <div className="dataset-actions">
        <button
          className="dataset-process-btn"
          onClick={(e) => {
            e.stopPropagation();
            if (!processingComplete) {
              onStartProcessing(dataset.name);
            }
          }}
          disabled={processingComplete}
          title={processingComplete ? 'Processing complete' : `Start processing "${dataset.name}"`}
          style={{
            opacity: processingComplete ? 0.6 : 1,
            cursor: processingComplete ? 'not-allowed' : 'pointer'
          }}
        >
          ▶
        </button>
        <button
          className="dataset-delete-btn"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(dataset.id, dataset.name);
          }}
          title={`Delete dataset "${dataset.name}"`}
        >
          ×
        </button>
      </div>
    </div>
  );
};

DatasetItem.propTypes = {
  dataset: PropTypes.object.isRequired,
  isSelected: PropTypes.bool.isRequired,
  onSelect: PropTypes.func.isRequired,
  onStartProcessing: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  inputFiles: PropTypes.array.isRequired,
  outputFiles: PropTypes.array.isRequired,
  processorType: PropTypes.string.isRequired
};

export default DatasetItem; 
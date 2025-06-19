import './App.css'

import { useState, useEffect } from 'react';

import Form from '@rjsf/core';
import validator from '@rjsf/validator-ajv8';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faFolder, faFile } from '@fortawesome/free-solid-svg-icons';

interface FileInfo {
  Name: string;
  IsDir: boolean;
  Url: string;
  Size: number;
  Language?: string;
  IsSelected?: boolean;
}

function App() {
  const [renderedFiles, setRenderedFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [schema, setSchema] = useState({});
  const [formValues, setFormValues] = useState({});
  const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);
  const [selectedFileContents, setSelectedFileContents] = useState("");

  // WRAPPER FUNCTIONS
  // ~~~

  // Define a wrapper function to handle async actions to standardize error handling and loading states
  const doAsyncAction = async(action: () => Promise<void>) => {
    try {
      setLoading(true);
      await action();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  // Define a wrapper function to log to the console
  const log = (type: string) => console.log.bind(console, type);


  // CORE FUNCTIONS
  // ~~~

  // Helper function to fetch the list of files rendered by boilerplate
  const fetchRenderedFiles = async (formData: any): Promise<FileInfo[]> => {
    const response = await fetch('http://localhost:8080/render', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(formData)
    });
    
    if (!response.ok) {
      throw new Error(`Failed to render files: ${response.statusText}`);
    }
    
    const result = await response.json();
    return result.files || [];
  };

  // Helper function to select the default file
  const selectDefaultFile = (files: FileInfo[]): FileInfo | null => {
    if (files.length === 0) return null;
    
    // Try to find a file marked as selected, otherwise use the last file
    return files.find(file => file.IsSelected) || files[files.length - 1];
  };

  // Define a function to render the files based on the form data
  const getRenderedFiles = async (data: { formData?: any }, event: React.FormEvent) => {
    await doAsyncAction(async () => {
      event.preventDefault();
      
      const formData = data.formData || {};
      setFormValues(formData);

      const files = await fetchRenderedFiles(formData);
      setRenderedFiles(files);

      const defaultSelectedFile = selectDefaultFile(files);
      setSelectedFile(defaultSelectedFile);

      const selectedFile = files.length > 0 ? files.find((file: FileInfo) => file.IsSelected) || files[files.length - 1] : null;
      setSelectedFile(selectedFile);
      await loadFile(selectedFile, event)
    });
  };

  // Define a function to load the file based on the form data
  const loadFile = async(file: FileInfo | null, event?: React.FormEvent) => {
    await doAsyncAction(async () => {
      event && event.preventDefault();
      setSelectedFile(file);
      if (file) {
        const response = await fetch(file.Url);
        const result = await response.text();
        setSelectedFileContents(result);
      }
    });
  };

  // REACT LIFECYCLE FUNCTIONS
  // ~~~

  // Helper function to fetch the form schema based on the boilerplate.yml template
  const fetchFormSchema = async () => {
    const response = await fetch(`http://localhost:8080/form`);
    const schema = await response.json();
    setSchema(schema);
  };

  // Fetch the form schema when the component mounts (when first starts rendering)
  useEffect(() => {
    async function init() {
      await doAsyncAction(async () => {
        fetchFormSchema();
      });
    }
    init();
  }, []);

  // MISCELLANEOUS FUNCTIONS
  // ~~~  

  // Customize some aspects of the React JSON Schema Form (RJSF) UI (e.g. button text)
  const uiSchema = {
    "ui:submitButtonOptions": {
      "submitText": "Generate"
    }
  };

  return (
    <div className="container">
      <div className="row justify-content-center">
        <div className="col-4 py-4">
          {error && (
              <div className="alert alert-danger" role="alert">
                <strong>Error</strong>: {error}
              </div>
          )}
          <Form schema={schema}
                validator={validator}
                uiSchema={uiSchema}
                formData={formValues}
                onSubmit={getRenderedFiles}
                onError={log("errors")} 
                />
        </div>
      

        {renderedFiles && renderedFiles.length > 0 && (
        <div className="col-8 py-4">
          {loading && (
              <div className="spinner-border" role="status">
                <span className="visually-hidden">Loading...</span>
              </div>
          )}
          {/* TODO: this table only renders one level of files and doesn't yet support nested files */}
          <table className="table table-hover">
            <thead>
              <tr>
                <th>File</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
            {renderedFiles.map(file =>
                <tr key={file.Name} className={file === selectedFile ? "table-active" : undefined}>
                  <td>
                    <FontAwesomeIcon icon={file.IsDir ? faFolder : faFile}/>
                    <a href="#" target="_blank" rel="noreferrer" className="mx-1" onClick={e => loadFile(file, e)}>
                      {file.Name}
                    </a>
                  </td>
                  <td>{file.Size}B</td>
                </tr>
            )}
            </tbody>
          </table>
          {selectedFile && selectedFileContents && (
            <div>
              <h5>Preview: <code>{selectedFile.Name}</code></h5>
              <pre><code className={`language-${selectedFile.Language}`}>{selectedFileContents}</code></pre>
            </div>
          )}
        </div>
      )}


      </div>
    </div>
  );
}

export default App

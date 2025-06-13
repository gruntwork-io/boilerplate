import './App.css'

import React, { useState, useEffect } from 'react'
import Form from '@rjsf/daisyui';
import validator from '@rjsf/validator-ajv8';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faFolder, faFile } from '@fortawesome/free-solid-svg-icons'

interface FileInfo {
  Name: string;
  IsDir: boolean;
  Url: string;
  Size: number;
  Language?: string;
  IsSelected?: boolean;
}

function App() {
  const [schema, setSchema] = useState({});
  const [renderedFiles, setRenderedFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [formValues, setFormValues] = useState({});
  const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);
  const [selectedFileContents, setSelectedFileContents] = useState("");

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

  const renderFiles = async (data: any, event: React.FormEvent) => {
    await doAsyncAction(async () => {
      event.preventDefault();
      setFormValues(data.formData);
      const response = await fetch('http://localhost:8080/render', {
        method: 'POST',
        body: JSON.stringify(data.formData)
      });
      const result = await response.json();

      const files = result['files'];
      setRenderedFiles(files);

      const selectedFile = files.length > 0 ? files.find((file: any) => file.IsSelected) || files[files.length - 1] : null;
      setSelectedFile(selectedFile);
      await loadFile(selectedFile, event)
    });
  };

  const loadFile = async(file: FileInfo, event?: React.SyntheticEvent) => {
    await doAsyncAction(async () => {
      event && event.preventDefault();
      setSelectedFile(file);
      const response = await fetch(file.Url);
      const result = await response.text();
      setSelectedFileContents(result);
    });
  };

  useEffect(() => {
    async function init() {
      await doAsyncAction(async () => {
        const response = await fetch(`http://localhost:8080/form`);
        const schema = await response.json();
        setSchema(schema);
      });
    }
    init();
  }, []);

  useEffect(() => {
    window.Prism.highlightAll();
  });

  const log = (type: string) => console.log.bind(console, type);

  const uiSchema = {
    "ui:submitButtonOptions": {
      "submitText": "Generate"
    }
  };

  return (
    <div className="container">
      <h2>Hello!</h2>
      <div className="row justify-content-center">
        <div className="col-4 py-4">
          {error && (
              <div className="alert alert-danger" role="alert">
                <strong>Error</strong>: {error}
              </div>
          )}
          <Form schema={schema}
                uiSchema={uiSchema}
                formData={formValues}
                onSubmit={renderFiles}
                onError={log("errors")}
                validator={validator} />
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
                    <a href={file.Url} target="_blank" rel="noreferrer" className="mx-1" onClick={e => loadFile(file, e)}>
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

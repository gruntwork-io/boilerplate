import './App.css'

import { useState, useEffect } from 'react';

import Form from '@rjsf/core';
import validator from '@rjsf/validator-ajv8';

interface FileInfo {
  Name: string;
  IsDir: boolean;
  Url: string;
  Size: number;
  Language?: string;
  IsSelected?: boolean;
}

function App() {
  // const [renderedFiles, setRenderedFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [schema, setSchema] = useState({});
  // const [formValues, setFormValues] = useState({});
  // const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);
  // const [selectedFileContents, setSelectedFileContents] = useState("");

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

  // Fetch the form schema based on the boilerplate.yml template
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
                // uiSchema={uiSchema}
                // formData={formValues}
                // onSubmit={renderFiles}
                // onError={log("errors")} 
                />
        </div>
      

      </div>
    </div>
  );
}

export default App

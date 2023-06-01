import './App.css';
import './Prism.css';
import React, { useEffect, useState } from "react";
import Form from "@rjsf/bootstrap-4";
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faFolder, faFile, faCircleInfo, faCirclePlay, faClipboard } from '@fortawesome/free-solid-svg-icons'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

function App() {
  const [formParts, setFormParts] = useState([]);
  const [renderedFiles, setRenderedFiles] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [formValues, setFormValues] = useState({});
  const [selectedFile, setSelectedFile] = useState(null);
  const [selectedFileContents, setSelectedFileContents] = useState("");

  const doAsyncAction = async(action) => {
    try {
      setLoading(true);
      await action();
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  };

  const renderFiles = async ({formData}, event) => {
    await doAsyncAction(async () => {
      event.preventDefault();
      setFormValues(formData);
      const response = await fetch('http://localhost:8080/render', {
        method: 'POST',
        body: JSON.stringify(formData)
      });
      const result = await response.json();

      const files = result['files'];
      setRenderedFiles(files);

      const selectedFile = files.length > 0 ? files.find(file => file.IsSelected) || files.find(file => file.Name.endsWith('.tf') || file.Name.endsWith('.hcl')) || files[files.length - 1] : null;
      await loadFile(selectedFile, event)
    });
  };

  const loadFile = async(file, event) => {
    await doAsyncAction(async () => {
      event && event.preventDefault();
      setSelectedFile(file);
      console.log(`FETCHING FROM URL '${file.Url}'`);
      const response = await fetch(file.Url);
      const result = await response.text();
      setSelectedFileContents(result);
    });
  };

  const fetchForm = async() => {
    const response = await fetch(`http://localhost:8080/form`);
    const parts = await response.json();
    setFormParts(parts);
  };

  const autoScaffold = async(modulePath) => {
    const response = await fetch(`http://localhost:8080/auto-scaffold/${modulePath}`);
    const parts = await response.json();
    setFormParts(parts);
  };

  useEffect(() => {
    async function init() {
      const autoScaffoldPath = "/auto-scaffold/"

      // There is a bug where this code seems to be called twice...
      if (window.location.pathname.startsWith(autoScaffoldPath)) {
        const modulePath = window.location.pathname.slice(autoScaffoldPath.length);
        await doAsyncAction(() => autoScaffold(modulePath));
      } else {
        await doAsyncAction(fetchForm);
      }
    }
    init();
  }, []);

  useEffect(() => {
    window.Prism.highlightAll();
  });

  const log = (type) => console.log.bind(console, type);

  const uiSchema = {
    "ui:submitButtonOptions": {
      "submitText": "Generate"
    },
    "root_AwsRegion": {
      classNames: "form-select"
    }
  };

  const renderFormPart = (part) => {
    // TODO: we should switch to string enums to avoid this awkward translation of int enum to string
    switch (part.Type) {
      case 0: // RawMarkdown
        return (
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{part.RawMarkdown}</ReactMarkdown>
        )
      case 1: // BoilerplateYaml
        // Ensure vars show up in proper order
        const uiSchemaForForm = Object.assign({}, uiSchema, {"ui:order": part.BoilerplateFormOrder});
        // TODO: we render a separate form for each part now... That works for examples with just one form, but
        // perhaps in the future we should support more types.
        return (
            <Form schema={part.BoilerplateYamlFormSchema}
                  uiSchema={uiSchemaForForm}
                  formData={formValues}
                  onSubmit={renderFiles}
                  onError={log("errors")} />
        )
      case 2: // BoilerplateTemplate
        if (renderedFiles && renderedFiles.length > 0) {
          const selectedFile = renderedFiles.find(file => file.Name === part.BoilerplateTemplatePath);
          if (!selectedFile) {
            throw new Error(`Could not find file '${part.BoilerplateTemplatePath}' in list of rendered files: ${JSON.stringify(renderedFiles)}`);
          }
          const url = new window.URL(part.BoilerplateTemplatePath, 'http://localhost:8080/rendered').href
          return (
              <div className="alert alert-success d-flex align-items-center" role="alert">
                <FontAwesomeIcon icon={faCircleInfo} className="me-2"/>
                <div>
                  View
                  <a href={url} onClick={e => loadFile(selectedFile, e)} className="mx-1"><code>{part.BoilerplateTemplatePath}</code></a>
                  in the right pane
                  &gt;&gt;&gt;
                </div>
              </div>
          )
        } else {
          return (
              <div className="alert alert-primary d-flex align-items-center" role="alert">
                <FontAwesomeIcon icon={faCircleInfo} className="me-2"/>
                <div>
                  Click "Generate" above to render: <code>{part.BoilerplateTemplatePath}</code>.
                </div>
              </div>
          )
        }
      case 3: // ExecutableSnippet
        // TODO: in the future, we could execute the code here by having the user press "play", but for now, we just render-as Markdown
        return (
            <div className="row gx-1 align-items-center">
              <div className="col-9">
                <pre><code className={`language-${part.ExecutableSnippetLang}`}>{part.ExecutableSnippet}</code></pre>
              </div>
              <div className="col-3">
                <button type="button" className="btn">
                  <FontAwesomeIcon icon={faCirclePlay} className="me-1"/>
                  <span style={{fontSize: "12px"}}>Run</span>
                </button>
                <button type="button" className="btn">
                  <FontAwesomeIcon icon={faClipboard} className="me-1"/>
                  <span style={{fontSize: "12px"}}>Copy</span>
                </button>
              </div>
            </div>
        )
      default:
        throw new Error(`Unrecognized part type '${part.Type}' in part: ${JSON.stringify(part)}`);
    }
  };

  return (
    <div className="container">
      <div className="row">
        <div className="col-5 py-4 markdown-container">
          {error && (
              <div className="alert alert-danger" role="alert">
                <strong>Error</strong>: {error}
              </div>
          )}
          {formParts.map(renderFormPart)}
        </div>
      {renderedFiles && renderedFiles.length > 0 && (
        <div className="col-6 py-4 fixed-top" style={{left: "46%"}}>
          {/* TODO: this table only renders one level of files and doesn't yet support nested files */}
          <table className="table table-hover">
            <thead>
              <tr>
                <th>File</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
            {/*Temporarily filter out folders to make it easier to show contents*/}
            {renderedFiles.filter(file => !file.IsDir).map(file =>
                <tr key={file.Name} className={file === selectedFile ? "table-active" : null}>
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
          {loading && (
              <div className="spinner-border" role="status">
                <span className="visually-hidden">Loading...</span>
              </div>
          )}
        </div>
      )}
      </div>
    </div>
  );
}

export default App;

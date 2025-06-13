// Make prismjs available to the window object
declare global {
    interface Window {
      Prism: {
        highlightAll: () => void;
        highlightElement: (element: Element) => void;
      };
    }
  }
  
  export {};
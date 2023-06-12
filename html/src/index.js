import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';
import reportWebVitals from './reportWebVitals';

const root = ReactDOM.createRoot(document.getElementById('root'));

const nav = [
    {url: "/", label: "Home"},
    {url: "/live", label: "Live"},
    {url: "/scaffolds", label: "Scaffolds"},
    {url: "/catalog", label: "Catalog"},
];

const navClass = "nav-item nav-link";
const navActiveClass = `${navClass} active`;

const isActiveNavItem = (navItem) => {
    const currentUrlNoSlash = window.location.pathname.slice(1).split('/')[0];
    const navItemUrlNoSlash = navItem.url.slice(1);

    console.log(`currentUrlNoSlash = ${currentUrlNoSlash} (length = ${currentUrlNoSlash.length}), navItemUrlNoSlash = ${navItemUrlNoSlash} (length = ${navItemUrlNoSlash.length})`)

    if (currentUrlNoSlash.length === 0) {
        return navItemUrlNoSlash.length === 0
    }

    if (currentUrlNoSlash === "auto-scaffold" && navItemUrlNoSlash === "catalog") {
        return true;
    }

    return navItemUrlNoSlash.startsWith(currentUrlNoSlash);
};

root.render(
    <React.StrictMode>
        <nav className="navbar navbar-dark navbar-expand-lg px-5 py-4 mb-4" style={{backgroundColor: "#33376d"}}>
            <div className="container">
                <a className="navbar-brand" href="/">
                    <img src="https://app.gruntwork.io/static/images/gruntwork-wordmark-light.png" width="179px"/>
                </a>
                <button className="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarNavAltMarkup" aria-controls="navbarNavAltMarkup" aria-expanded="false" aria-label="Toggle navigation">
                    <span className="navbar-toggler-icon"></span>
                </button>
                <div className="collapse navbar-collapse px-5" id="navbarNavAltMarkup">
                    <div className="navbar-nav">
                        {nav.map(navItem => <a className={isActiveNavItem(navItem) ? navActiveClass : navClass} href={navItem.url} key={navItem.url}>{navItem.label}</a>)}
                    </div>
                </div>
                <img src="https://lh3.googleusercontent.com/a-/AOh14Gi7fkfB4WpPg4CC5mO40yiD5tjvsb9jyjIzx5xXJA=s96-c" alt="..." className="rounded-circle" width="48px"/>
            </div>
        </nav>
        <App />
    </React.StrictMode>
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();

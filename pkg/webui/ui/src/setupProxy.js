const { createProxyMiddleware } = require('http-proxy-middleware');

module.exports = function(app) {
    app.use(
        '/staticdata',
        createProxyMiddleware({
            target: 'http://localhost:9090',
            changeOrigin: true,
        })
    );
    app.use(
        '/api',
        createProxyMiddleware({
            target: 'http://localhost:9090',
            changeOrigin: true,
        })
    );
    app.use(
        '/auth',
        createProxyMiddleware({
            target: 'http://localhost:9090',
            changeOrigin: true,
        })
    );
};
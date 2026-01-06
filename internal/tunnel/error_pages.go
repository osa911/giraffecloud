package tunnel

import (
	"fmt"
	"net"
)

// WriteNotConnectedPage writes a user-friendly "Tunnel Not Connected" error page
func WriteNotConnectedPage(conn net.Conn, domain string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tunnel Not Connected | GiraffeCloud</title>
    <style>
        :root {
            --primary: #F5A623;
            --bg: #0a0a0a;
            --surface: #1a1a1a;
            --text: #ffffff;
            --text-dim: #888888;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: var(--bg);
            color: var(--text);
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .container {
            background-color: var(--surface);
            padding: 40px;
            border-radius: 12px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.4);
            max-width: 480px;
            text-align: center;
            border: 1px solid #333;
        }
        h1 {
            color: var(--primary);
            margin-bottom: 16px;
            font-size: 24px;
        }
        p {
            color: var(--text-dim);
            line-height: 1.6;
            margin-bottom: 24px;
        }
        .domain {
            background: #000;
            padding: 8px 12px;
            border-radius: 6px;
            font-family: monospace;
            color: #fff;
            margin-bottom: 24px;
            display: inline-block;
        }
        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            background: rgba(245, 166, 35, 0.1);
            color: var(--primary);
            padding: 8px 16px;
            border-radius: 100px;
            font-size: 14px;
            font-weight: 500;
            margin-bottom: 24px;
        }
        .status-dot {
            width: 8px;
            height: 8px;
            background-color: var(--primary);
            border-radius: 50%%;
            box-shadow: 0 0 8px var(--primary);
        }
        .btn {
            display: inline-block;
            background-color: var(--primary);
            color: #000;
            text-decoration: none;
            padding: 12px 24px;
            border-radius: 6px;
            font-weight: 600;
            transition: opacity 0.2s;
        }
        .btn:hover {
            opacity: 0.9;
        }
        .refresh-hint {
            margin-top: 24px;
            font-size: 13px;
            color: #555;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="status-badge">
            <div class="status-dot"></div>
            Tunnel Offline
        </div>
        <h1>Tunnel Not Connected</h1>
        <p>The tunnel you are trying to access is currently offline. The client application is not connected to our servers.</p>
        <div class="domain">%s</div>
        <p>If you are the owner of this tunnel, please ensure your GiraffeCloud CLI is running.</p>
        <a href="javascript:location.reload()" class="btn">Try Again</a>
        <div class="refresh-hint">Auto-refreshing in 30 seconds...</div>
    </div>
    <script>
        setTimeout(() => location.reload(), 30000);
    </script>
</body>
</html>`, domain)

	response := fmt.Sprintf("HTTP/1.1 503 Service Unavailable\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"Retry-After: 30\r\n"+
		"\r\n"+
		"%s", len(html), html)

	conn.Write([]byte(response))
}

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Issue #270</title>
    <style>
        .container {
            width: 90%;
            margin: 0 auto;
            padding: 16px;
            background-color: #f5f5f5;
            border-radius: 4px;
            margin-bottom: 20px;
        }

        .count-list {
            margin: 0;
            padding: 0;
            list-style-type: none;
        }

        .count-list li {
            font-size: 14px;
            color: #64748b;
            margin-bottom: 5px;
        }

        .element-style {
            color: #999999;
            font-size: 12px;
            font-weight: 400;
            margin: 0;
            margin-bottom: 3px;
        }

        .feed-url {
            font-size: 15px;
            margin-bottom: 15px;
        }

        a {
            text-decoration: none;
            color: #2563eb;
        }

        a:hover {
            text-decoration: underline;
        }

        .dashboard-table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 15px;
            font-size: 14px;
            font-family: monospace;
        }

        .dashboard-table th,
        .dashboard-table td {
            padding: 8px;
            text-align: left;
            border: 1px solid #ddd;
            vertical-align: top;
        }

        .dashboard-table th {
            background-color: #f8f9fa;
            font-weight: 600;
        }

        .dashboard-table tr:hover {
            background-color: #f5f5f5;
        }

        .dashboard-table a {
            color: #2563eb;
            text-decoration: none;
        }

        .dashboard-table a:hover {
            text-decoration: underline;
        }

        .dashboard-section {
            margin-bottom: 20px;
        }

        .dashboard-section h3 {
            color: #374151;
            margin-bottom: 10px;
            font-size: 18px;
        }

        .feed-count {
            color: #64748b;
            font-weight: 600;
        }

        .feed-type-header {
            font-size: 16px;
            font-weight: 600;
            color: #374151;
            margin: 20px 0 10px 0;
            padding: 8px;
            background-color: #f8f9fa;
            border-radius: 4px;
            border-left: 4px solid #2563eb;
        }

        /* 添加折叠面板样式 */
        details {
            margin: 10px 0;
            padding: 10px;
            background-color: #ffffff;
            border-radius: 4px;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
        }

        details > summary {
            padding: 8px;
            font-weight: 600;
            color: #374151;
            cursor: pointer;
            user-select: none;
        }

        details > summary:hover {
            background-color: #f8f9fa;
            border-radius: 4px;
        }

        details[open] > summary {
            margin-bottom: 10px;
            border-bottom: 1px solid #e5e7eb;
        }
    </style>
</head>
<body>
    {{if .DashboardHTML}}
    <div class="container">
        <h2>Dashboard</h2>
        {{ .DashboardHTML | safeHTML }}
    </div>
    {{end}}

    <div class="container">
        <h1>Issue In This Week:</h1>
        <ul class="count-list">
            {{range .Feeds}}
            <li>{{len .Items}} from {{.Category}}</li>
            {{end}}
        </ul>
    </div>

    {{range .Feeds}}
    <div class="container">
        <h2>{{.Category}}</h2>
        {{range .Items}}
        <div class="element-style">{{.PubDate}}</div>
        <div class="feed-url"><a href="{{.Link}}" target="_blank">{{.Title}}</a></div>
        {{end}}
    </div>
    {{end}}
</body>
</html>

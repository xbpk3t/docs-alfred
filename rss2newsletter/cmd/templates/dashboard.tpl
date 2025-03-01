<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Dashboard For RSS Feeds</title>
    <style>
        .container {
            width: 90%;
            margin: 0 auto;
            padding: 16px;
            background-color: #f5f5f5;
            border-radius: 4px;
            margin-bottom: 20px;
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
    </style>
</head>
<body>
    <div class="container">
        <h2>Dashboard For RSS Feeds</h2>
        {{ .DashboardHTML | safeHTML }}
    </div>
</body>
</html>

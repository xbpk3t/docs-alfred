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
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Issue In This Week:</h1>
        <ul class="count-list">
            {{range .}}
            <li>{{len .URLs}} from {{.Type}}</li>
            {{end}}
        </ul>
    </div>

    {{range .}}
    <div class="container">
        <h2>{{.Type}}</h2>
        {{range .URLs}}
        <div class="element-style">{{.Date}}</div>
        <div class="feed-url"><a href="{{.URL}}" target="_blank">{{.Des}}</a></div>
        {{end}}
    </div>
    {{end}}
</body>
</html>

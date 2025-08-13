# Axora Search with MongoDB Storage

A search application using DuckDuckGo Lite with MongoDB NoSQL storage for search results.

## Features

- **Search**: Interactive CLI for web searches using DuckDuckGo Lite
- **Storage**: MongoDB NoSQL database for storing search results
- **Integration**: Seamless integration between search and storage components

## Project Structure

```
axora/
├── cli/
│   └── cli.py              # Interactive search CLI
├── search/
│   └── search.py           # DuckDuckGo search implementation
├── storage/
│   ├── __init__.py
│   ├── database.py         # MongoDB connection and operations
├── requirements.txt        # Python dependencies
└── README.md               # This file
```

## Setup
   ```
1. **Install Python dependencies**:
   ```bash
   pip install -r requirements.txt
   ```

## Usage

### 1. Use the Search CLI

```bash
python cli/cli.py
```

Available commands:
- Type any query to search
- `help` - Show help information
- `set max_urls <number>` - Set maximum URLs to fetch
- `quit` - Exit the CLI

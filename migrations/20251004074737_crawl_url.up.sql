-- Create crawl_url table
CREATE TABLE IF NOT EXISTS crawl_url (
    id UUID PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    is_downloadable BOOLEAN NOT NULL DEFAULT false,
    download_status VARCHAR(50) NOT NULL
);
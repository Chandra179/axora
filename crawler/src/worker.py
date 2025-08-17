"""
Implements the pipeline for a single URL: 
- normalize → claim → robots check → host politeness → fetch → classify → save → publish.

URL normalization/canonicalization happens in the normalize step:
- Convert to lowercase, remove fragments, normalize paths
- Handle redirects and canonical URLs
- Generate URL fingerprint for deduplication

URL deduplication happens in the claim step:
- Check if normalized URL already exists in MongoDB
- Use URL fingerprint or hash for fast lookups
- Skip processing if URL was recently crawled

Called by consumer.py.
"""
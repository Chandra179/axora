import structlog
import trafilatura
import json
from typing import Optional
from lxml import html

logger = structlog.get_logger(__name__)


def extract_content(url: str) -> Optional[dict]:
    try:
        downloaded = trafilatura.fetch_url(url)
        
        if not downloaded:
            logger.warning("download_failed", url=url)
            return None
        
        result = trafilatura.extract(
            downloaded,
            output_format='json',
            include_comments=False,  
            include_tables=True,     
            with_metadata=True,          
            include_links=True,
            url=url
        )
        
        if not result:
            logger.warning("trafilatura_extraction_failed", url=url)
            return None
        
        data = json.loads(result)
        
        return {
            "title": data.get("title"),
            "url": url,
            "author": data.get("author"),
            "hostname": data.get("hostname"),
            "date": data.get("date"),
            "text": data.get("text"),
            "language": data.get("language"),
            "source": data.get("source"),
            "source_hostname": data.get("source-hostname"),
            "excerpt": data.get("excerpt"),
            "categories": data.get("categories"),
        }
        
    except json.JSONDecodeError as e:
        logger.error("json_decode_error", 
                    url=url,
                    error=str(e),
                    exc_info=True)
        return None
    
    except Exception as e:
        logger.error("extraction_error", 
                    url=url,
                    error=str(e),
                    exc_info=True)
        return None
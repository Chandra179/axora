"""
Test script for content extraction pipeline
Run without Kafka to test the extraction logic
"""

import logging
from url_validator import validate_url
from web_extractor import extract_content
from content_validator import validate_content_quality

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

# Test URLs
test_urls = [
    "https://www.bbc.com/news/business",  # News article
    "https://techcrunch.com/",             # Tech news
]

def test_extraction(url):
    """Test the full extraction pipeline"""
    print("\n" + "="*80)
    print(f"Testing URL: {url}")
    print("="*80)
    
    # Step 1: Validate
    print("\n1. Validating URL...")
    validation = validate_url(url)
    print(f"   Valid: {validation['valid']}")
    print(f"   Reason: {validation['reason']}")
    
    if not validation['valid']:
        print("   ✗ Skipping (validation failed)")
        return
    
    # Step 2: Extract
    print("\n2. Extracting content...")
    extracted = extract_content(url)
    
    if not extracted:
        print("   ✗ Extraction failed")
        return
    
    print(f"   ✓ Extracted {extracted['word_count']} words")
    print(f"   Title: {extracted['title'][:60]}...")
    
    # Step 3: Validate quality
    print("\n3. Validating quality...")
    quality = validate_content_quality(extracted)
    print(f"   Passed: {quality['passed']}")
    print(f"   Score: {quality['quality_score']:.1f}/100")
    
    if quality['passed']:
        print("\n✓ SUCCESS - Content is ready for processing")
        print(f"\nFirst 200 characters of content:")
        print("-" * 80)
        print(extracted['text'][:200] + "...")
    else:
        print("\n✗ FAILED - Quality checks not met:")
        for reason in quality['reasons']:
            print(f"   - {reason}")


if __name__ == "__main__":
    print("\n" + "="*80)
    print("CONTENT EXTRACTION TEST")
    print("="*80)
    
    for url in test_urls:
        test_extraction(url)
        print("\n")
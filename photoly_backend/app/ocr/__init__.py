"""On-demand CPU OCR, independent of embedding models."""
LANGUAGES = 'chi_sim+chi_tra+eng+jpn'
MAX_BYTES = 20 * 1024 * 1024
MAX_PIXELS = 40_000_000
MAX_TEXT = 100_000
TOTAL_SECONDS = 45
ENGINE_SECONDS = 30
FORMATS = {'JPEG', 'PNG', 'WEBP', 'GIF', 'TIFF', 'BMP', 'ICO', 'PPM'}

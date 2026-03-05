from typing import Dict, Any, Optional
from .client import OpenVK

def add_skill(data: Any, wait: bool = False, timeout: Optional[float] = None) -> Dict[str, Any]:
    """Add skill to OpenViking."""
    client = OpenVK.get_client()
    return client.add_skill(data, wait=wait, timeout=timeout)

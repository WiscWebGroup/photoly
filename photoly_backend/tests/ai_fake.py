"""Only present in test images; never advertised as a real model backend."""
import numpy as np
from app.ai.types import EmbeddingBatch


class FakeEmbedding:
    def download(self, spec, progress):
        import os, time
        progress("downloading", 0, 2)
        time.sleep(float(os.getenv("AI_TEST_DOWNLOAD_SECONDS", "0")))
        progress("downloading", 2, 2)
    def load(self,spec,config):
        import os, time
        time.sleep(float(os.getenv("AI_TEST_LOAD_SECONDS", "0")))
        self.spec=spec
    def encode_texts(self,texts,*,role):
        if isinstance(texts[0],str) and texts[0]=="__test_oom__":raise RuntimeError("out of memory")
        if isinstance(texts[0],str) and texts[0]=="__test_timeout__":
            import time
            time.sleep(2)
        value=np.zeros((len(texts),self.spec["dimension"]),dtype=np.float32)
        value[:,0]=1
        return EmbeddingBatch(value,self.spec["dimension"],self.spec["fingerprint"])
    def encode_images(self,images,*,role):return self.encode_texts(images,role=role)
    def close(self):pass

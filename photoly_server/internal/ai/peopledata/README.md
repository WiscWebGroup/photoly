# Pinned People model artifacts

OpenCV Zoo revision `48b10fa505aeb51fbd127fc60a5c660040e823b7`.
Profile fingerprint `yunet2023-sface2021-rgb-v1`. Files are fetched lazily, not committed as binaries.

| Role | File | Bytes | SHA-256 | License |
| --- | --- | ---: | --- | --- |
| Detector | face_detection_yunet_2023mar.onnx | 232589 | 8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4 | LICENSE.YuNet (MIT) |
| Embedding | face_recognition_sface_2021dec.onnx | 38696353 | 0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79 | LICENSE.SFace (Apache-2.0) |

Upstream directories at the pinned revision:
- https://github.com/opencv/opencv_zoo/tree/48b10fa505aeb51fbd127fc60a5c660040e823b7/models/face_detection_yunet
- https://github.com/opencv/opencv_zoo/tree/48b10fa505aeb51fbd127fc60a5c660040e823b7/models/face_recognition_sface

Runtime source: `people_model.go`. Preprocessing contract and evaluation limitations: `docs/docs/administration/people.md`. Preserve upstream license texts with redistributed model files.

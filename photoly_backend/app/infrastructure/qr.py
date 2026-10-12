from io import BytesIO
import qrcode


class QRCodeEncoder:
    def encode(self, value):
        stream = BytesIO()
        qrcode.make(value).resize((250, 250)).save(stream, format="PNG")
        stream.seek(0)
        return stream

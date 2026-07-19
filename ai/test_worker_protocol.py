import io
import json
import unittest

from ai.worker_protocol import iter_requests, write_response


class RecordingStream(io.StringIO):
    def __init__(self):
        super().__init__()
        self.flush_count = 0

    def flush(self):
        self.flush_count += 1
        super().flush()


class WorkerProtocolTests(unittest.TestCase):
    def test_accepts_limit_and_crlf(self):
        stream = io.BytesIO(b"12345678\r\nnext\n")

        self.assertEqual(
            list(iter_requests(stream, max_bytes=8)),
            [("12345678", None), ("next", None)],
        )

    def test_drains_oversized_line_before_next_request(self):
        stream = io.BytesIO(b"123456789012345\nnext\n")

        requests = list(iter_requests(stream, max_bytes=8))

        self.assertEqual(requests[0], (None, "request exceeds 8 bytes"))
        self.assertEqual(requests[1], ("next", None))

    def test_rejects_invalid_utf8(self):
        stream = io.BytesIO(b"\xff\n")

        self.assertEqual(
            list(iter_requests(stream)),
            [(None, "request is not valid UTF-8")],
        )

    def test_response_is_single_line_json_and_flushes(self):
        stream = RecordingStream()

        write_response({"message": "first\nsecond"}, stream)

        self.assertEqual(json.loads(stream.getvalue()), {"message": "first\nsecond"})
        self.assertEqual(stream.getvalue().count("\n"), 1)
        self.assertEqual(stream.flush_count, 1)

    def test_non_finite_response_becomes_protocol_error(self):
        stream = RecordingStream()

        write_response({"score": float("nan")}, stream)

        response = json.loads(stream.getvalue())
        self.assertIn("response serialization failed", response["error"])


if __name__ == "__main__":
    unittest.main()

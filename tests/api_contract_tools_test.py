#!/usr/bin/env python3
from __future__ import annotations

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))
sys.path.insert(0, str(ROOT / "scripts" / "ci"))

import build_openapi  # noqa: E402
import check_proto_breaking  # noqa: E402
import openapi_refs  # noqa: E402
import openapi_verify_release  # noqa: E402


class OpenAPIContractToolTests(unittest.TestCase):
    def test_unresolved_local_refs_reports_missing_components(self) -> None:
        spec = {
            "openapi": "3.0.3",
            "components": {"schemas": {"Present": {"type": "object"}}},
            "paths": {
                "/v1/example": {
                    "get": {
                        "responses": {
                            "200": {
                                "description": "ok",
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/Missing"},
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }

        missing = openapi_refs.unresolved_local_refs(spec)

        self.assertEqual(1, len(missing))
        self.assertIn("#/components/schemas/Missing", missing[0])

    def test_non_local_refs_detects_external_refs(self) -> None:
        spec = {
            "openapi": "3.0.3",
            "paths": {
                "/x": {
                    "get": {
                        "responses": {
                            "200": {
                                "description": "ok",
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "other.json#/definitions/Foo"},
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }
        ext = openapi_refs.non_local_refs(spec)
        self.assertEqual(1, len(ext))
        self.assertIn("other.json", ext[0][1])

    def test_verify_paths_reports_required_operation_drift(self) -> None:
        method, path = build_openapi.REQUIRED_OPERATIONS[0]
        paths = {path: {"post" if method == "get" else "get": {}}}

        missing = build_openapi.verify_paths(paths)

        self.assertIn(f"{method.upper()} {path}", missing)

    def test_release_verifier_treats_password_reset_as_public(self) -> None:
        self.assertEqual({"post"}, openapi_verify_release.NO_BEARER["/v1/auth/password/reset/request"])
        self.assertEqual({"post"}, openapi_verify_release.NO_BEARER["/v1/auth/password/reset/confirm"])

    def test_proto_breaking_normalizes_windows_paths(self) -> None:
        self.assertEqual("proto/avf/machine/v1/auth.proto", check_proto_breaking.normalized(r".\proto\avf\machine\v1\auth.proto"))

    def test_duplicate_operation_ids_detects_collision(self) -> None:
        paths = {
            "/a": {"get": {"operationId": "DocOpDup"}},
            "/b": {"post": {"operationId": "DocOpDup"}},
        }
        dups = openapi_refs.duplicate_operation_ids(paths)
        self.assertEqual(1, len(dups))
        self.assertIn("DocOpDup", dups[0])


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Seed the Northridge District client via Huma HTTP APIs and run today's pipelines."""

from __future__ import annotations

import json
import sys
import time
import urllib.error
import urllib.request

API = "http://172.20.0.5:8000"
ADMIN_EMAIL = "northridge.admin@example.com"
ADMIN_PASSWORD = "Northridge1"
ORG_EMAIL = "registrar@riverside.example"
ORG_PASSWORD = "Northridge1"


class APIError(RuntimeError):
    def __init__(self, status: int, path: str, body: str):
        super().__init__(f"{status} {path}: {body}")
        self.status = status
        self.path = path
        self.body = body


class Client:
    def __init__(self) -> None:
        self.token: str | None = None

    def request(self, method: str, path: str, body: object | None = None, expected: tuple[int, ...] = (200, 201)):
        data = None
        headers = {"Accept": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(API + path, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                raw = resp.read()
                payload = json.loads(raw) if raw else None
                if resp.status not in expected:
                    raise APIError(resp.status, path, raw.decode() if raw else "")
                return payload
        except urllib.error.HTTPError as err:
            raw = err.read().decode() if err.fp else ""
            if err.code not in expected:
                raise APIError(err.code, path, raw) from err
            return json.loads(raw) if raw else None

    def login_user(self, email: str, password: str) -> None:
        tokens = self.request("POST", "/auth/login/user", {"email": email, "password": password})
        self.token = tokens["accessToken"]

    def login_org(self, email: str, password: str, network_id: str, organization_id: str) -> None:
        tokens = self.request(
            "POST",
            "/auth/login/organization-user",
            {
                "email": email,
                "password": password,
                "networkId": network_id,
                "organizationId": organization_id,
            },
        )
        self.token = tokens["accessToken"]


def items(payload) -> list:
    if payload is None:
        return []
    if isinstance(payload, list):
        return payload
    return payload.get("items") or []


def find_by(payload, key: str, value: str):
    for item in items(payload):
        if item.get(key) == value:
            return item
    return None


def schema_doc(title: str, description: str, slug: str, properties: dict, required: list[str]) -> dict:
    return {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "$id": f"https://schemas.lutia.dev/{slug}",
        "title": title,
        "description": description,
        "type": "object",
        "additionalProperties": False,
        "properties": properties,
        "required": required,
    }


def main() -> int:
    admin = Client()
    created = admin.request(
        "POST",
        "/user",
        {
            "firstName": "Northridge",
            "lastName": "Admin",
            "email": ADMIN_EMAIL,
            "password": ADMIN_PASSWORD,
        },
        expected=(201, 409),
    )
    print("admin user:", "created" if created else "exists")
    admin.login_user(ADMIN_EMAIL, ADMIN_PASSWORD)

    networks = items(admin.request("GET", "/network"))
    network = next((n for n in networks if n.get("name") == "Northridge District"), None)
    if network is None:
        network_id = admin.request("POST", "/network", {"name": "Northridge District"})["id"]
        print("network created", network_id)
    else:
        network_id = network["id"]
        print("network exists", network_id)

    org_names = [
        "District Office",
        "Maple Elementary",
        "Cedar Middle",
        "Riverside High",
    ]
    org_ids: dict[str, str] = {}
    existing_orgs = items(admin.request("GET", f"/organization?networkId={network_id}"))
    for name in org_names:
        found = next((o for o in existing_orgs if o.get("name") == name), None)
        if found:
            org_ids[name] = found["id"]
            print("org exists", name, found["id"])
            continue
        org_id = admin.request("POST", "/organization", {"name": name, "networkId": network_id})["id"]
        org_ids[name] = org_id
        print("org created", name, org_id)

    high_id = org_ids["Riverside High"]
    org_users = items(
        admin.request("GET", f"/organization-user?networkId={network_id}&organizationId={high_id}")
    )
    registrar = next((u for u in org_users if u.get("email") == ORG_EMAIL), None)
    if registrar is None:
        admin.request(
            "POST",
            "/organization-user",
            {
                "firstName": "Riley",
                "lastName": "Registrar",
                "email": ORG_EMAIL,
                "password": ORG_PASSWORD,
                "networkId": network_id,
                "organizationId": high_id,
            },
        )
        print("registrar created")
    else:
        print("registrar exists", registrar["id"])

    schemas = items(admin.request("GET", f"/schema?networkId={network_id}"))
    schema_ids: dict[str, str] = {s["slug"]: s["id"] for s in schemas}

    def ensure_schema(name: str, slug: str, definition: dict) -> str:
        if slug in schema_ids:
            print("schema exists", slug, schema_ids[slug])
            return schema_ids[slug]
        schema_id = admin.request(
            "POST",
            "/schema",
            {"name": name, "networkId": network_id, "definition": definition},
        )["id"]
        schema_ids[slug] = schema_id
        print("schema created", slug, schema_id)
        return schema_id

    enrollment_id = ensure_schema(
        "Enrollment",
        "enrollment",
        schema_doc(
            "Enrollment",
            "Student placement at a campus.",
            "enrollment",
            {
                "studentId": {"type": "string", "description": "District student number"},
                "givenName": {"type": "string"},
                "familyName": {"type": "string"},
                "gradeLevel": {
                    "type": "string",
                    "enum": ["K", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"],
                },
                "campus": {"type": "string", "enum": ["elementary", "middle", "high"]},
                "status": {
                    "type": "string",
                    "enum": ["enrolled", "waitlisted", "withdrawn"],
                    "default": "enrolled",
                },
                "enrolledOn": {"type": "string", "format": "date"},
                "homeroom": {"type": "string"},
                "guardianName": {"type": "string"},
                "guardianEmail": {"type": "string", "format": "email"},
                "guardianPhone": {"type": "string"},
                "transcriptFileId": {"type": "string", "format": "file"},
            },
            ["studentId", "givenName", "familyName", "gradeLevel", "campus", "status"],
        ),
    )
    attendance_id = ensure_schema(
        "Attendance",
        "attendance",
        schema_doc(
            "Attendance",
            "Daily attendance marks.",
            "attendance",
            {
                "enrollmentId": {
                    "type": "string",
                    "format": "foreign",
                    "schemaId": enrollment_id,
                },
                "studentId": {"type": "string"},
                "campus": {"type": "string", "enum": ["elementary", "middle", "high"]},
                "status": {"type": "string", "enum": ["present", "absent", "tardy", "excused"]},
                "markedOn": {"type": "string", "format": "date"},
                "guardianEmail": {"type": "string", "format": "email"},
                "guardianName": {"type": "string"},
            },
            ["studentId", "campus", "status", "markedOn"],
        ),
    )
    ensure_schema(
        "Course Section",
        "course-section",
        schema_doc(
            "Course Section",
            "Course sections published across campuses.",
            "course-section",
            {
                "courseId": {"type": "string"},
                "title": {"type": "string"},
                "teacher": {"type": "string"},
                "campus": {"type": "string", "enum": ["elementary", "middle", "high"]},
                "period": {"type": "integer"},
                "seats": {"type": "integer"},
                "gradeBand": {"type": "string", "enum": ["K-5", "6-8", "9-12"]},
            },
            ["courseId", "title", "campus"],
        ),
    )
    notice_id = ensure_schema(
        "Notice",
        "notice",
        schema_doc(
            "Notice",
            "Family and staff notices.",
            "notice",
            {
                "kind": {
                    "type": "string",
                    "enum": ["onboarding", "absence", "credentials", "report-card", "transfer"],
                },
                "channel": {"type": "string", "enum": ["family", "counselor", "registrar"]},
                "status": {
                    "type": "string",
                    "enum": ["pending", "sent", "failed"],
                    "default": "pending",
                },
                "enrollmentId": {
                    "type": "string",
                    "format": "foreign",
                    "schemaId": enrollment_id,
                },
                "studentId": {"type": "string"},
                "to": {"type": "string"},
                "subject": {"type": "string"},
                "body": {"type": "string"},
            },
            ["kind", "channel", "status"],
        ),
    )
    transfer_id = ensure_schema(
        "Transfer Request",
        "transfer-request",
        schema_doc(
            "Transfer Request",
            "Campus transfer requests.",
            "transfer-request",
            {
                "enrollmentId": {
                    "type": "string",
                    "format": "foreign",
                    "schemaId": enrollment_id,
                },
                "studentId": {"type": "string"},
                "fromCampus": {"type": "string", "enum": ["elementary", "middle", "high"]},
                "toCampus": {"type": "string", "enum": ["elementary", "middle", "high"]},
                "reason": {"type": "string"},
                "status": {
                    "type": "string",
                    "enum": ["requested", "approved", "denied"],
                    "default": "requested",
                },
            },
            ["enrollmentId", "fromCampus", "toCampus", "status"],
        ),
    )

    pipelines = items(admin.request("GET", f"/pipeline-definition?networkId={network_id}"))
    pipeline_ids: dict[str, str] = {p["slug"]: p["id"] for p in pipelines}

    def ensure_pipeline(name: str, slug: str, description: str, nodes: list) -> str:
        if slug in pipeline_ids:
            print("pipeline exists", slug, pipeline_ids[slug])
            return pipeline_ids[slug]
        pipeline_id = admin.request(
            "POST",
            "/pipeline-definition",
            {
                "name": name,
                "description": description,
                "active": True,
                "networkId": network_id,
                "definition": {"nodes": nodes},
            },
        )["id"]
        pipeline_ids[slug] = pipeline_id
        print("pipeline created", slug, pipeline_id)
        return pipeline_id

    sis_id = ensure_pipeline(
        "SIS Ingest",
        "sis-ingest",
        "Pull JSONPlaceholder users and create enrollment records.",
        [
            [
                {
                    "name": "Pull enrollments",
                    "type": "HTTP",
                    "definition": {
                        "method": "GET",
                        "url": "https://jsonplaceholder.typicode.com/users",
                    },
                }
            ],
            [
                {
                    "name": "Create enrollments",
                    "type": "BULK",
                    "definition": {
                        "operation": "CREATE",
                        "records": [
                            {
                                "schemaId": enrollment_id,
                                "from": "{{ .Input.0.body }}",
                                "as": "user",
                                "data": {
                                    "studentId": "{{ printf \"%.0f\" .user.id }}",
                                    "givenName": "{{ .user.name }}",
                                    "familyName": "{{ .user.username }}",
                                    "gradeLevel": "11",
                                    "campus": "high",
                                    "status": "enrolled",
                                    "enrolledOn": "2026-08-18",
                                    "guardianName": "{{ .user.name }}",
                                    "guardianEmail": "{{ .user.email }}",
                                    "guardianPhone": "{{ .user.phone }}",
                                },
                            }
                        ],
                    },
                }
            ],
        ],
    )
    attendance_pipe_id = ensure_pipeline(
        "Attendance Capture",
        "attendance-capture",
        "Pull JSONPlaceholder todos and create attendance marks.",
        [
            [
                {
                    "name": "Ingest marks",
                    "type": "HTTP",
                    "definition": {
                        "method": "GET",
                        "url": "https://jsonplaceholder.typicode.com/todos",
                    },
                }
            ],
            [
                {
                    "name": "Create attendance",
                    "type": "BULK",
                    "definition": {
                        "operation": "CREATE",
                        "records": [
                            {
                                "schemaId": attendance_id,
                                "from": "{{ .Input.0.body }}",
                                "as": "todo",
                                "data": {
                                    "studentId": "{{ printf \"%.0f\" .todo.userId }}",
                                    "campus": "high",
                                    "status": "{{ if .todo.completed }}present{{ else }}absent{{ end }}",
                                    "markedOn": "{{ or .Input.date \"2026-09-11\" }}",
                                },
                            }
                        ],
                    },
                }
            ],
        ],
    )
    creds_id = ensure_pipeline(
        "Issue Credentials",
        "issue-credentials",
        "Create a student portal account from JSONPlaceholder.",
        [
            [
                {
                    "name": "Create account",
                    "type": "HTTP",
                    "definition": {
                        "method": "POST",
                        "url": "https://jsonplaceholder.typicode.com/users",
                        "body": {
                            "username": "{{ .Input.studentId }}",
                            "name": "{{ .Input.givenName }} {{ .Input.familyName }}",
                            "email": "{{ .Input.guardianEmail }}",
                        },
                    },
                }
            ],
            [
                {
                    "name": "Notify family",
                    "type": "RECORD",
                    "definition": {
                        "operation": "CREATE",
                        "schemaId": notice_id,
                        "data": {
                            "kind": "credentials",
                            "channel": "family",
                            "status": "pending",
                            "enrollmentId": "{{ .Input.enrollmentId }}",
                            "studentId": "{{ .Input.studentId }}",
                            "to": "{{ .Input.guardianEmail }}",
                            "subject": "Student portal account ready",
                            "body": "Account created for student {{ .Input.studentId }}.",
                        },
                    },
                }
            ],
        ],
    )
    gradebook_id = ensure_pipeline(
        "Gradebook Sync",
        "gradebook-sync",
        "Pull JSONPlaceholder posts as term grades.",
        [
            [
                {
                    "name": "Pull grades",
                    "type": "HTTP",
                    "definition": {
                        "method": "GET",
                        "url": "https://jsonplaceholder.typicode.com/posts?userId={{ .Input.studentId }}",
                    },
                }
            ],
            [
                {
                    "name": "Queue family notice",
                    "type": "RECORD",
                    "definition": {
                        "operation": "CREATE",
                        "schemaId": notice_id,
                        "data": {
                            "kind": "report-card",
                            "channel": "family",
                            "status": "pending",
                            "enrollmentId": "{{ .Input.enrollmentId }}",
                            "studentId": "{{ .Input.studentId }}",
                            "to": "{{ .Input.guardianEmail }}",
                            "subject": "Report card available",
                            "body": "Term grades are ready for student {{ .Input.studentId }}.",
                        },
                    },
                }
            ],
        ],
    )

    saved = admin.request("GET", f"/pipeline-definition/{sis_id}")
    first_node = saved["definition"]["nodes"][0][0]
    print("sis http node", json.dumps(first_node, indent=2))
    if first_node.get("type") != "HTTP" or "jsonplaceholder.typicode.com/users" not in first_node.get("definition", {}).get("url", ""):
        raise RuntimeError("SIS HTTP node did not persist")

    workflows = items(admin.request("GET", f"/workflow-definition?networkId={network_id}"))
    workflow_ids = {w["slug"]: w["id"] for w in workflows}

    def ensure_workflow(name: str, slug: str, description: str, schema_id: str, definition: dict) -> str:
        if slug in workflow_ids:
            print("workflow exists", slug, workflow_ids[slug])
            return workflow_ids[slug]
        workflow_id = admin.request(
            "POST",
            "/workflow-definition",
            {
                "name": name,
                "description": description,
                "active": True,
                "schemaId": schema_id,
                "networkId": network_id,
                "definition": definition,
            },
        )["id"]
        workflow_ids[slug] = workflow_id
        print("workflow created", slug, workflow_id)
        return workflow_id

    ensure_workflow(
        "Student Onboarding",
        "student-onboarding",
        "Place a new student and issue credentials.",
        enrollment_id,
        {
            "trigger": {"on": ["created"]},
            "criteria": {"field": "status", "operator": "eq", "value": "enrolled"},
            "actions": [
                {
                    "type": "CREATE_RECORD",
                    "context": {
                        "schemaId": notice_id,
                        "data": {
                            "kind": "onboarding",
                            "channel": "counselor",
                            "status": "pending",
                            "enrollmentId": "{{ .Record.id }}",
                            "studentId": "{{ .Record.data.studentId }}",
                            "subject": "New enrollment {{ .Record.data.givenName }}",
                            "body": "Assign homeroom at {{ .Record.data.campus }}, grade {{ .Record.data.gradeLevel }}.",
                        },
                    },
                },
                {
                    "type": "TRIGGER_PIPELINE",
                    "context": {
                        "pipeline": creds_id,
                        "input": {
                            "enrollmentId": "{{ .Record.id }}",
                            "studentId": "{{ .Record.data.studentId }}",
                            "givenName": "{{ .Record.data.givenName }}",
                            "familyName": "{{ .Record.data.familyName }}",
                            "campus": "{{ .Record.data.campus }}",
                            "guardianEmail": "{{ .Record.data.guardianEmail }}",
                        },
                    },
                },
            ],
        },
    )
    ensure_workflow(
        "Absence Notice",
        "absence-notice",
        "Notify family when a student is marked absent.",
        attendance_id,
        {
            "trigger": {"on": ["created"]},
            "criteria": {"field": "status", "operator": "eq", "value": "absent"},
            "actions": [
                {
                    "type": "CREATE_RECORD",
                    "context": {
                        "schemaId": notice_id,
                        "data": {
                            "kind": "absence",
                            "channel": "family",
                            "status": "pending",
                            "studentId": "{{ .Record.data.studentId }}",
                            "subject": "Absence recorded {{ .Record.data.markedOn }}",
                            "body": "Student {{ .Record.data.studentId }} was marked absent.",
                        },
                    },
                }
            ],
        },
    )
    ensure_workflow(
        "Report Card",
        "report-card",
        "Sync gradebook at 6am Denver time.",
        enrollment_id,
        {
            "trigger": {"on": ["schedule"], "cron": "0 6 * * *", "timezone": "America/Denver"},
            "criteria": {"field": "status", "operator": "eq", "value": "enrolled"},
            "actions": [
                {
                    "type": "TRIGGER_PIPELINE",
                    "context": {
                        "pipeline": gradebook_id,
                        "input": {
                            "enrollmentId": "{{ .Record.id }}",
                            "studentId": "{{ .Record.data.studentId }}",
                            "campus": "{{ .Record.data.campus }}",
                            "guardianEmail": "{{ .Record.data.guardianEmail }}",
                        },
                    },
                }
            ],
        },
    )
    ensure_workflow(
        "Transfer Request",
        "transfer-request",
        "Flag registrar and waitlist the student.",
        transfer_id,
        {
            "trigger": {"on": ["created"]},
            "criteria": {"field": "status", "operator": "eq", "value": "requested"},
            "actions": [
                {
                    "type": "CREATE_RECORD",
                    "context": {
                        "schemaId": notice_id,
                        "data": {
                            "kind": "transfer",
                            "channel": "registrar",
                            "status": "pending",
                            "enrollmentId": "{{ .Record.data.enrollmentId }}",
                            "studentId": "{{ .Record.data.studentId }}",
                            "subject": "Transfer {{ .Record.data.fromCampus }} to {{ .Record.data.toCampus }}",
                        },
                    },
                },
                {
                    "type": "UPDATE_RECORD",
                    "context": {
                        "schemaId": enrollment_id,
                        "recordId": "{{ .Record.data.enrollmentId }}",
                        "data": {"status": "waitlisted"},
                    },
                },
            ],
        },
    )

    staff = Client()
    staff.login_org(ORG_EMAIL, ORG_PASSWORD, network_id, high_id)

    def run_pipeline(definition_id: str, name: str, extra_input: dict | None = None) -> str:
        payload = {"pipelineDefinitionId": definition_id, "input": extra_input or {}}
        run_id = staff.request("POST", "/pipeline", payload)["id"]
        print("enqueued", name, run_id)
        for _ in range(40):
            run = staff.request("GET", f"/pipeline/{run_id}")
            status = run.get("status")
            print(" ", name, status)
            if status in {"completed", "failed"}:
                if status == "failed":
                    print("  error", run.get("error"))
                    nodes = staff.request("GET", f"/pipeline/{run_id}/node")
                    print("  nodes", json.dumps(nodes, indent=2)[:4000])
                return run_id
            time.sleep(1.5)
        raise RuntimeError(f"{name} did not finish")

    run_pipeline(sis_id, "sis-ingest")
    run_pipeline(attendance_pipe_id, "attendance-capture", {"date": "2026-09-11"})

    enrollments = items(
        staff.request(
            "GET",
            f"/record?schemaId={enrollment_id}&networkId={network_id}&organizationId={high_id}&pageSize=100",
        )
    )
    first = next(
        (
            rec
            for rec in enrollments
            if str((rec.get("data") or {}).get("studentId")) == "1"
        ),
        enrollments[0] if enrollments else None,
    )
    if first is not None:
        data = first.get("data") or {}
        run_pipeline(
            gradebook_id,
            "gradebook-sync",
            {
                "studentId": str(data.get("studentId") or "1"),
                "enrollmentId": first["id"],
                "guardianEmail": data.get("guardianEmail") or "Sincere@april.biz",
            },
        )

    time.sleep(3)
    pipes = items(staff.request("GET", "/pipeline?pageSize=50"))
    wfs = items(staff.request("GET", "/workflow?pageSize=50"))
    recs = items(staff.request("GET", f"/record?networkId={network_id}&organizationId={high_id}&pageSize=100"))
    print("pipeline runs", len(pipes), [(p.get("status"), p.get("id")) for p in pipes[:12]])
    print("workflow runs", len(wfs), [(w.get("status"), w.get("name", w.get("id"))) for w in wfs[:12]])
    print("records", len(recs))
    print("login", ADMIN_EMAIL, "org user", ORG_EMAIL)
    print("network", network_id)
    print("high school", high_id)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except APIError as err:
        print(err, file=sys.stderr)
        sys.exit(1)

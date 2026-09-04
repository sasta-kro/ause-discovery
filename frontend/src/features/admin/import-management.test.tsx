// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../../app/i18n";
import { AdminImportReview } from "./import-management";

const apiMocks = vi.hoisted(() => ({
  getImport: vi.fn(),
  listImportRows: vi.fn(),
  updateImportRows: vi.fn(),
  commitImport: vi.fn(),
  getImportResult: vi.fn(),
  createImport: vi.fn(),
}));

vi.mock("../../api/generated/sdk.gen", () => apiMocks);

const batchID = "018f0000-0000-7000-8000-000000000901";
const projectID = "018f0000-0000-7000-8000-000000000902";
const baseBatch = {
  id: batchID,
  source_filename: "projects.csv",
  source_sha256: "a".repeat(64),
  format: "csv",
  state: "ready",
  total_rows: 1,
  valid_rows: 0,
  warning_rows: 1,
  error_rows: 0,
  revision: 2,
  created_at: "2026-09-04T00:00:00Z",
  expires_at: "2026-09-05T00:00:00Z",
};
const baseRow = {
  row_number: 1,
  import_key: "row-001",
  state: "warning",
  selected: false,
  warnings_acknowledged: false,
  draft: {
    project: {
      title: "Imported Project",
      reference_code: "REF-001",
      academic_year: 2026,
    },
  },
  issues: [
    {
      code: "project_duplicate_candidate",
      severity: "warning",
      message: "Possible duplicate.",
    },
  ],
  duplicate_candidates: [
    {
      id: projectID,
      reference_code: "REF-OLD",
      title: "Existing Project",
      academic_year: 2026,
      semester: "first",
      program: { id: projectID, key: "computing", label: "Computing" },
      course: { id: projectID, key: "capstone", label: "Capstone" },
      categories: [],
      platforms: [],
      people: [],
      artifact_count: 0,
      published_at: "2026-09-01T00:00:00Z",
    },
  ],
};

function renderReview() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[`/admin/imports/${batchID}`]}>
          <Routes>
            <Route
              path="/admin/imports/:batchId"
              element={<AdminImportReview csrfToken="csrf-token" />}
            />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  );
}

describe("Import management", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMocks.getImport
      .mockResolvedValueOnce({ data: baseBatch })
      .mockResolvedValue({ data: { ...baseBatch, revision: 3 } });
    apiMocks.listImportRows
      .mockResolvedValueOnce({
        data: { items: [baseRow], page: { limit: 100 } },
      })
      .mockResolvedValue({
        data: {
          items: [
            {
              ...baseRow,
              state: "selected",
              selected: true,
              warnings_acknowledged: true,
              duplicate_resolution: "create",
            },
          ],
          page: { limit: 100 },
        },
      });
    apiMocks.updateImportRows.mockResolvedValue({
      data: { ...baseBatch, revision: 3 },
    });
    apiMocks.commitImport.mockResolvedValue({
      data: {
        batch_id: batchID,
        committed_at: "2026-09-04T01:00:00Z",
        created_project_ids: [projectID],
        skipped_rows: [],
      },
    });
    apiMocks.getImportResult.mockResolvedValue({
      data: {
        batch_id: batchID,
        committed_at: "2026-09-04T01:00:00Z",
        created_project_ids: [projectID],
        skipped_rows: [],
      },
    });
  });

  it("persists warning and duplicate decisions before committing selected rows", async () => {
    const user = userEvent.setup();
    renderReview();

    await screen.findByText("Imported Project");
    await user.click(screen.getByRole("checkbox", { name: "Select row 1" }));
    await user.click(
      screen.getByRole("checkbox", { name: "Acknowledge warnings" }),
    );
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Duplicate action for row 1" }),
      "create",
    );
    await user.click(
      screen.getByRole("button", { name: "Save row decisions" }),
    );

    await waitFor(() =>
      expect(apiMocks.updateImportRows).toHaveBeenCalledWith(
        expect.objectContaining({
          body: {
            expected_revision: 2,
            rows: [
              {
                row_number: 1,
                selected: true,
                acknowledge_warnings: true,
                duplicate_resolution: "create",
              },
            ],
          },
          headers: { "X-CSRF-Token": "csrf-token" },
        }),
      ),
    );
    await user.click(
      screen.getByRole("button", { name: "Commit selected rows" }),
    );
    await waitFor(() =>
      expect(apiMocks.commitImport).toHaveBeenCalledWith(
        expect.objectContaining({ body: { expected_revision: 3 } }),
      ),
    );
    expect(await screen.findByText("Import committed")).toBeTruthy();
    expect(screen.getByRole("link", { name: projectID })).toBeTruthy();
  });
});

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  commitImport,
  createImport,
  getImport,
  getImportResult,
  listImportRows,
  updateImportRows,
} from "../../api/generated/sdk.gen";
import type { ImportRow, ImportRowUpdate } from "../../api/generated/types.gen";
import styles from "../../App.module.css";

const maximumImportBytes = 25 * 1024 * 1024;

export function AdminImportUpload({ csrfToken }: { csrfToken: string | null }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [file, setFile] = useState<File | null>(null);
  const [clientError, setClientError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken || !file)
        throw new Error("Import upload prerequisites are unavailable");
      const extension = file.name.toLowerCase().split(".").pop();
      if (extension !== "csv" && extension !== "xlsx")
        throw new Error(t("imports.invalidType"));
      if (file.size > maximumImportBytes)
        throw new Error(t("imports.tooLarge"));
      return (
        await createImport({
          body: { file },
          headers: { "X-CSRF-Token": csrfToken },
          throwOnError: true,
        })
      ).data;
    },
    onSuccess: (batch) => navigate(`/admin/imports/${batch.id}`),
    onError: (error) =>
      setClientError(
        error instanceof Error ? error.message : t("imports.failed"),
      ),
  });

  return (
    <div>
      <div className={styles.pageHeader}>
        <h1>{t("imports.title")}</h1>
        <p>{t("imports.description")}</p>
      </div>
      <section className={styles.panel}>
        <h2>{t("imports.new")}</h2>
        <form
          className={styles.form}
          onSubmit={(event) => {
            event.preventDefault();
            setClientError(null);
            mutation.mutate();
          }}
        >
          <label className={styles.field}>
            <span>{t("imports.file")}</span>
            <input
              accept=".csv,.xlsx"
              required
              type="file"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
          </label>
          <button
            className={styles.button}
            disabled={!csrfToken || !file || mutation.isPending}
            type="submit"
          >
            {t("imports.preview")}
          </button>
        </form>
        {clientError ? (
          <p className={styles.error} role="alert">
            {clientError}
          </p>
        ) : null}
      </section>
      <section className={styles.detailSection}>
        <h2>{t("imports.templates")}</h2>
        <p>{t("imports.templatesHelp")}</p>
        <div className={styles.formActions}>
          <a
            className={styles.secondaryButton}
            download
            href={`${import.meta.env.BASE_URL}templates/ause-discovery-import.csv`}
          >
            {t("imports.csvTemplate")}
          </a>
          <a
            className={styles.secondaryButton}
            download
            href={`${import.meta.env.BASE_URL}templates/ause-discovery-import.xlsx`}
          >
            {t("imports.xlsxTemplate")}
          </a>
        </div>
        <details>
          <summary>{t("imports.schema")}</summary>
          <p>
            <strong>CSV:</strong>{" "}
            <code>
              import_key, title, reference_code, abstract, academic_year,
              semester, program_key, major_key, course_key, title_aliases,
              students, advisors, co_advisors, committee_members, categories,
              platforms, domains, topics, technologies
            </code>
          </p>
          <p>
            <strong>XLSX Projects:</strong>{" "}
            <code>
              import_key, title, reference_code, abstract, academic_year,
              semester, program_key, major_key, course_key, title_aliases
            </code>
          </p>
          <p>
            <strong>XLSX Participations:</strong>{" "}
            <code>import_key, role, display_name, student_id, staff_id</code>
          </p>
          <p>
            <strong>XLSX Classifications:</strong>{" "}
            <code>import_key, dimension, key</code>
          </p>
        </details>
      </section>
    </div>
  );
}

type RowDecision = ImportRowUpdate;

function rowDecision(row: ImportRow): RowDecision {
  return {
    row_number: row.row_number,
    selected: row.selected,
    acknowledge_warnings: row.warnings_acknowledged,
    duplicate_resolution: row.duplicate_resolution,
  };
}

export function AdminImportReview({ csrfToken }: { csrfToken: string | null }) {
  const { t } = useTranslation();
  const { batchId = "" } = useParams();
  const queryClient = useQueryClient();
  const [cursor, setCursor] = useState<string | undefined>();
  const [cursorHistory, setCursorHistory] = useState<Array<string | undefined>>(
    [],
  );
  const [decisions, setDecisions] = useState<Record<number, RowDecision>>({});
  const [decisionsDirty, setDecisionsDirty] = useState(false);
  const batchQuery = useQuery({
    queryKey: ["admin-import", batchId],
    queryFn: async () =>
      (await getImport({ path: { batch_id: batchId }, throwOnError: true }))
        .data,
  });
  const rowsQuery = useQuery({
    queryKey: ["admin-import-rows", batchId, cursor],
    queryFn: async () =>
      (
        await listImportRows({
          path: { batch_id: batchId },
          query: { limit: 100, cursor },
          throwOnError: true,
        })
      ).data,
  });
  const resultQuery = useQuery({
    queryKey: ["admin-import-result", batchId],
    queryFn: async () =>
      (
        await getImportResult({
          path: { batch_id: batchId },
          throwOnError: true,
        })
      ).data,
    enabled: batchQuery.data?.state === "committed",
  });

  useEffect(() => {
    if (!rowsQuery.data) return;
    setDecisions(
      Object.fromEntries(
        rowsQuery.data.items.map((row) => [row.row_number, rowDecision(row)]),
      ),
    );
    setDecisionsDirty(false);
  }, [rowsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken || !batchQuery.data)
        throw new Error("Import update prerequisites are unavailable");
      return (
        await updateImportRows({
          path: { batch_id: batchId },
          body: {
            expected_revision: batchQuery.data.revision,
            rows: Object.values(decisions),
          },
          headers: { "X-CSRF-Token": csrfToken },
          throwOnError: true,
        })
      ).data;
    },
    onSuccess: async (batch) => {
      queryClient.setQueryData(["admin-import", batchId], batch);
      setDecisionsDirty(false);
      await queryClient.invalidateQueries({
        queryKey: ["admin-import-rows", batchId],
      });
    },
  });
  const commitMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken || !batchQuery.data)
        throw new Error("Import commit prerequisites are unavailable");
      return (
        await commitImport({
          path: { batch_id: batchId },
          body: { expected_revision: batchQuery.data.revision },
          headers: { "X-CSRF-Token": csrfToken },
          throwOnError: true,
        })
      ).data;
    },
    onSuccess: async (result) => {
      queryClient.setQueryData(["admin-import-result", batchId], result);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["admin-import", batchId] }),
        queryClient.invalidateQueries({
          queryKey: ["admin-import-rows", batchId],
        }),
      ]);
    },
  });
  const selectValidMutation = useMutation({
    mutationFn: async () => {
      if (!csrfToken || !batchQuery.data)
        throw new Error("Import update prerequisites are unavailable");
      // Rows are paginated, so walk every page and select each row whose
      // server state is valid. Warning and error rows keep needing per-row
      // acknowledgement or source correction.
      const updates: ImportRowUpdate[] = [];
      let pageCursor: string | undefined;
      do {
        const page = (
          await listImportRows({
            path: { batch_id: batchId },
            query: { limit: 100, cursor: pageCursor },
            throwOnError: true,
          })
        ).data;
        for (const row of page.items) {
          if (row.state !== "valid") continue;
          updates.push({
            row_number: row.row_number,
            selected: true,
            acknowledge_warnings: row.warnings_acknowledged,
            duplicate_resolution: row.duplicate_resolution,
          });
        }
        pageCursor = page.page.next_cursor ?? undefined;
      } while (pageCursor);
      if (!updates.length) return batchQuery.data;
      return (
        await updateImportRows({
          path: { batch_id: batchId },
          body: {
            expected_revision: batchQuery.data.revision,
            rows: updates,
          },
          headers: { "X-CSRF-Token": csrfToken },
          throwOnError: true,
        })
      ).data;
    },
    onSuccess: async (batch) => {
      queryClient.setQueryData(["admin-import", batchId], batch);
      await queryClient.invalidateQueries({
        queryKey: ["admin-import-rows", batchId],
      });
    },
  });
  const rows = rowsQuery.data?.items ?? [];
  const unresolved = rows.some((row) => {
    const decision = decisions[row.row_number];
    if (!decision?.selected) return false;
    return (
      (row.issues.some((issue) => issue.severity === "warning") &&
        !decision.acknowledge_warnings) ||
      ((row.duplicate_candidates?.length ?? 0) > 0 &&
        !decision.duplicate_resolution)
    );
  });
  const failed =
    batchQuery.isError ||
    rowsQuery.isError ||
    saveMutation.isError ||
    selectValidMutation.isError ||
    commitMutation.isError ||
    resultQuery.isError;
  const result = commitMutation.data ?? resultQuery.data;

  if (batchQuery.isPending || rowsQuery.isPending)
    return <p role="status">{t("feedback.loading")}</p>;
  return (
    <div>
      <div className={styles.resultsHeader}>
        <div>
          <h1>{t("imports.review")}</h1>
          <p className={styles.metadata}>{batchQuery.data?.source_filename}</p>
        </div>
        <Link className={styles.secondaryButton} to="/admin/imports">
          {t("imports.new")}
        </Link>
      </div>
      {batchQuery.data ? (
        <section
          className={styles.importSummary}
          aria-label={t("imports.summary")}
        >
          <strong>{t(`imports.state.${batchQuery.data.state}`)}</strong>
          <span>
            {t("imports.total", { count: batchQuery.data.total_rows })}
          </span>
          <span>
            {t("imports.valid", { count: batchQuery.data.valid_rows })}
          </span>
          <span>
            {t("imports.warnings", { count: batchQuery.data.warning_rows })}
          </span>
          <span>
            {t("imports.errors", { count: batchQuery.data.error_rows })}
          </span>
        </section>
      ) : null}
      {failed ? (
        <p className={styles.error} role="alert">
          {t("imports.failed")}
        </p>
      ) : null}
      {result ? (
        <section className={styles.notice} role="status">
          <h2>{t("imports.committed")}</h2>
          <p>
            {t("imports.commitSummary", {
              created: result.created_project_ids.length,
              skipped: result.skipped_rows.length,
            })}
          </p>
          {result.created_project_ids.length ? (
            <ul>
              {result.created_project_ids.map((projectId) => (
                <li key={projectId}>
                  <Link to={`/admin/projects/${projectId}/edit`}>
                    {projectId}
                  </Link>
                </li>
              ))}
            </ul>
          ) : null}
        </section>
      ) : null}
      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>{t("imports.select")}</th>
              <th>{t("imports.row")}</th>
              <th>{t("imports.project")}</th>
              <th>{t("imports.issues")}</th>
              <th>{t("imports.resolution")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <ImportRowEditor
                key={row.row_number}
                row={row}
                decision={decisions[row.row_number] ?? rowDecision(row)}
                disabled={batchQuery.data?.state !== "ready"}
                onChange={(decision) => {
                  setDecisions((current) => ({
                    ...current,
                    [row.row_number]: decision,
                  }));
                  setDecisionsDirty(true);
                }}
              />
            ))}
          </tbody>
        </table>
      </div>
      <div className={styles.formActions}>
        <button
          className={styles.secondaryButton}
          disabled={!cursorHistory.length}
          type="button"
          onClick={() => {
            const history = [...cursorHistory];
            setCursor(history.pop());
            setCursorHistory(history);
          }}
        >
          {t("imports.previous")}
        </button>
        <button
          className={styles.secondaryButton}
          disabled={!rowsQuery.data?.page.next_cursor}
          type="button"
          onClick={() => {
            setCursorHistory((history) => [...history, cursor]);
            setCursor(rowsQuery.data?.page.next_cursor ?? undefined);
          }}
        >
          {t("imports.next")}
        </button>
      </div>
      {batchQuery.data?.state === "ready" ? (
        <div className={styles.formActions}>
          <button
            className={styles.secondaryButton}
            disabled={
              !csrfToken ||
              !batchQuery.data.valid_rows ||
              saveMutation.isPending ||
              selectValidMutation.isPending
            }
            type="button"
            onClick={() => selectValidMutation.mutate()}
          >
            {t("imports.selectAllValid")}
          </button>
          <button
            className={styles.button}
            disabled={
              !csrfToken ||
              !rows.length ||
              !decisionsDirty ||
              unresolved ||
              saveMutation.isPending
            }
            type="button"
            onClick={() => saveMutation.mutate()}
          >
            {t("imports.saveSelection")}
          </button>
          <button
            className={styles.dangerButton}
            disabled={
              !csrfToken ||
              decisionsDirty ||
              commitMutation.isPending ||
              saveMutation.isPending
            }
            type="button"
            onClick={() => commitMutation.mutate()}
          >
            {t("imports.commit")}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function ImportRowEditor({
  row,
  decision,
  disabled,
  onChange,
}: {
  row: ImportRow;
  decision: RowDecision;
  disabled: boolean;
  onChange: (decision: RowDecision) => void;
}) {
  const { t } = useTranslation();
  const project = row.draft?.project as Record<string, unknown> | undefined;
  const hasWarnings = row.issues.some((issue) => issue.severity === "warning");
  const hasErrors = row.issues.some((issue) => issue.severity === "error");
  const candidates = row.duplicate_candidates ?? [];
  return (
    <tr>
      <td>
        <label>
          <span className="sr-only">
            {t("imports.selectRow", { row: row.row_number })}
          </span>
          <input
            aria-label={t("imports.selectRow", { row: row.row_number })}
            checked={decision.selected}
            disabled={disabled || hasErrors}
            type="checkbox"
            onChange={(event) =>
              onChange({ ...decision, selected: event.target.checked })
            }
          />
        </label>
      </td>
      <td>
        <strong>{row.row_number}</strong>
        <div className={styles.metadata}>
          {row.import_key} · {row.state}
        </div>
      </td>
      <td>
        <strong>{String(project?.title ?? row.import_key)}</strong>
        <div className={styles.metadata}>
          {String(project?.reference_code ?? "")}{" "}
          {String(project?.academic_year ?? "")}
        </div>
      </td>
      <td>
        {row.issues.length ? (
          <ul className={styles.issueList}>
            {row.issues.map((issue, index) => (
              <li
                key={`${issue.code}-${index}`}
                className={
                  issue.severity === "error" ? styles.fieldError : undefined
                }
              >
                {issue.message ?? issue.code}
              </li>
            ))}
          </ul>
        ) : (
          t("imports.noIssues")
        )}
      </td>
      <td>
        {hasWarnings ? (
          <label className={styles.checkboxLabel}>
            <input
              checked={decision.acknowledge_warnings}
              disabled={disabled}
              type="checkbox"
              onChange={(event) =>
                onChange({
                  ...decision,
                  acknowledge_warnings: event.target.checked,
                })
              }
            />
            {t("imports.acknowledge")}
          </label>
        ) : null}
        {candidates.length ? (
          <label className={styles.field}>
            <span>{t("imports.duplicateAction", { row: row.row_number })}</span>
            <select
              aria-label={t("imports.duplicateAction", { row: row.row_number })}
              disabled={disabled}
              value={decision.duplicate_resolution ?? ""}
              onChange={(event) =>
                onChange({
                  ...decision,
                  duplicate_resolution: event.target.value
                    ? (event.target.value as "create" | "skip")
                    : undefined,
                })
              }
            >
              <option value="">{t("imports.choose")}</option>
              <option value="create">{t("imports.createDuplicate")}</option>
              <option value="skip">{t("imports.skipDuplicate")}</option>
            </select>
          </label>
        ) : null}
        {candidates.map((candidate) => (
          <Link key={candidate.id} to={`/admin/projects/${candidate.id}/edit`}>
            {candidate.title}
          </Link>
        ))}
      </td>
    </tr>
  );
}

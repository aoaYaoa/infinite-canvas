import { apiPost } from "@/services/api/request";
import { useUserStore } from "@/stores/use-user-store";

export async function deleteCanvasTasks(sourceId: string, nodeIds: string[] = []) {
    const token = useUserStore.getState().token;
    const source = sourceId.trim();
    if (!token || !source) return;
    return apiPost<{ deleted: boolean }>(
        "/api/v1/canvas/tasks/delete",
        {
            source_id: source,
            node_ids: Array.from(new Set(nodeIds.map((id) => id.trim()).filter(Boolean))),
        },
        token,
    );
}

export async function deleteCanvasProjects(ids: string[]) {
    const token = useUserStore.getState().token;
    const projectIds = Array.from(
        new Set(ids.map((id) => id.trim()).filter(Boolean)),
    );
    if (!token || !projectIds.length) return;
    return apiPost<{ deleted: boolean }>(
        "/api/v1/canvas/projects/delete",
        { ids: projectIds },
        token,
    );
}

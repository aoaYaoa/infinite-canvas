import { create } from "zustand";
import { persist, type PersistStorage, type StorageValue } from "zustand/middleware";

import { nanoid } from "nanoid";
import { localForageStorage } from "@/lib/localforage-storage";
import { fetchUserConfig, syncUserCanvasData } from "@/services/api/user-config";
import { useUserStore } from "@/stores/use-user-store";
import type { CanvasBackgroundMode } from "@/lib/canvas-theme";
import type { CanvasAssistantSession, CanvasConnection, CanvasNodeData, ViewportTransform } from "../types";

export type CanvasProject = {
    id: string;
    title: string;
    createdAt: string;
    updatedAt: string;
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    chatSessions: CanvasAssistantSession[];
    activeChatId: string | null;
    backgroundMode: CanvasBackgroundMode;
    showImageInfo: boolean;
    viewport: ViewportTransform;
};

type CanvasStore = {
    hydrated: boolean;
    projects: CanvasProject[];
    createProject: (title?: string) => string;
    importProject: (project: Partial<CanvasProject>) => string;
    openProject: (id: string) => CanvasProject | null;
    renameProject: (id: string, title: string) => void;
    deleteProjects: (ids: string[]) => void;
    updateProject: (id: string, patch: Partial<Pick<CanvasProject, "nodes" | "connections" | "chatSessions" | "activeChatId" | "backgroundMode" | "showImageInfo" | "viewport">>) => void;
    syncWithRemote: (token: string, remoteData: any, syncEnabled: boolean) => Promise<void>;
    setSyncEnabled: (enabled: boolean) => void;
};

const initialViewport: ViewportTransform = { x: 0, y: 0, k: 1 };
const CANVAS_STORE_KEY = "infinite-canvas:canvas_store";
type PersistedCanvasState = Pick<CanvasStore, "projects">;
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let queuedPersistState: PersistedCanvasState | null = null;
let accountCanvasSyncEnabled = false;

const canvasStorage: PersistStorage<CanvasStore> = {
    getItem: async (name) => {
        const token = useUserStore.getState().token;
        const localValue = await localForageStorage.getItem(name);
        const localParsed = localValue
            ? (JSON.parse(localValue) as StorageValue<CanvasStore>)
            : null;
        const localProjects =
            (localParsed?.state as PersistedCanvasState)?.projects || [];
        const localHasData =
            Array.isArray(localProjects) && localProjects.length > 0;
        if (token) {
            try {
                const userConfig = await fetchUserConfig(token);
                accountCanvasSyncEnabled =
                    userConfig.syncCapabilities?.userData === true;
                const remote =
                    userConfig.canvasData as PersistedCanvasState | undefined;
                const remoteProjects = Array.isArray(remote?.projects)
                    ? remote.projects
                    : [];
                const remoteHasData = remoteProjects.length > 0;
                if (accountCanvasSyncEnabled && localHasData) {
                    const synced = await syncUserCanvasData<PersistedCanvasState>(
                        token,
                        { projects: localProjects },
                    ).catch(() => ({
                        projects: mergeCanvasProjects(remoteProjects, localProjects),
                    }));
                    const nextState = {
                        projects: Array.isArray(synced?.projects)
                            ? synced.projects
                            : remoteProjects,
                    };
                    const parsed = {
                        state: nextState,
                        version: 0,
                    } as StorageValue<CanvasStore>;
                    queuedPersistState = nextState;
                    await localForageStorage.setItem(
                        name,
                        JSON.stringify(parsed),
                    );
                    return parsed;
                }
                if (remoteHasData && (accountCanvasSyncEnabled || !localHasData)) {
                    const nextState = { projects: remoteProjects };
                    const parsed = {
                        state: nextState,
                        version: 0,
                    } as StorageValue<CanvasStore>;
                    queuedPersistState = nextState;
                    await localForageStorage.setItem(
                        name,
                        JSON.stringify(parsed),
                    );
                    return parsed;
                }
            } catch (error) {
                console.error(
                    "Failed to hydrate canvas projects from remote",
                    error,
                );
            }
        }
        if (!localParsed) return null;
        queuedPersistState = localParsed.state as PersistedCanvasState;
        if (token && accountCanvasSyncEnabled) {
            void syncUserCanvasData(
                token,
                localParsed.state as PersistedCanvasState,
            ).catch(() => undefined);
        }
        return localParsed;
    },

    setItem: (name, value) => {
        const nextState = value.state as PersistedCanvasState;
        if (
            queuedPersistState &&
            queuedPersistState.projects === nextState.projects
        ) {
            return;
        }
        queuedPersistState = nextState;
        if (saveTimer) clearTimeout(saveTimer);
        saveTimer = setTimeout(() => {
            saveTimer = null;
            void localForageStorage.setItem(name, JSON.stringify(value));
            const token = useUserStore.getState().token;
            if (token && accountCanvasSyncEnabled) {
                void syncUserCanvasData(token, nextState).catch(
                    () => undefined,
                );
            }
        }, 400);
    },
    removeItem: (name) => localForageStorage.removeItem(name),
};

export const useCanvasStore = create<CanvasStore>()(
    persist(
        (set, get) => ({
            hydrated: false,
            projects: [],
            createProject: (title = "未命名画布") => {
                const now = new Date().toISOString();
                const id = nanoid();
                const project: CanvasProject = {
                    id,
                    title,
                    createdAt: now,
                    updatedAt: now,
                    nodes: [],
                    connections: [],
                    chatSessions: [],
                    activeChatId: null,
                    backgroundMode: "lines",
                    showImageInfo: false,
                    viewport: initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                return id;
            },
            importProject: (source) => {
                const now = new Date().toISOString();
                const project: CanvasProject = {
                    id: nanoid(),
                    title: source.title || "导入画布",
                    createdAt: source.createdAt || now,
                    updatedAt: now,
                    nodes: source.nodes || [],
                    connections: source.connections || [],
                    chatSessions: source.chatSessions || [],
                    activeChatId: source.activeChatId || null,
                    backgroundMode: source.backgroundMode || "lines",
                    showImageInfo: source.showImageInfo || false,
                    viewport: source.viewport || initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                return project.id;
            },
            openProject: (id) => {
                return get().projects.find((item) => item.id === id) || null;
            },
            renameProject: (id, title) =>
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, title: title.trim() || project.title, updatedAt: new Date().toISOString() } : project)),
                })),
            deleteProjects: (ids) =>
                set((state) => ({
                    projects: state.projects.filter(
                        (project) => !ids.includes(project.id),
                    ),
                })),
            updateProject: (id, patch) =>
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, ...patch, updatedAt: new Date().toISOString() } : project)),
                })),
            syncWithRemote: async (token, remoteData, syncEnabled) => {
                accountCanvasSyncEnabled = syncEnabled;
                if (!syncEnabled) return;

                const localProjects = get().projects;
                const remoteProjects = Array.isArray(remoteData?.projects)
                    ? remoteData.projects
                    : [];
                const synced = localProjects.length
                    ? await syncUserCanvasData<PersistedCanvasState>(
                        token,
                        { projects: localProjects },
                    ).catch(() => ({
                        projects: mergeCanvasProjects(remoteProjects, localProjects),
                    }))
                    : { projects: remoteProjects };
                const nextState = {
                    projects: Array.isArray(synced.projects)
                        ? synced.projects
                        : remoteProjects,
                };
                queuedPersistState = nextState;
                set(nextState);
                await localForageStorage.setItem(
                    CANVAS_STORE_KEY,
                    JSON.stringify({ state: nextState, version: 0 }),
                );
            },
            setSyncEnabled: (enabled) => {
                accountCanvasSyncEnabled = enabled;
            },
        }),
        {
            name: CANVAS_STORE_KEY,
            storage: canvasStorage,
            partialize: (state) =>
                ({
                    projects: state.projects,
                }) as StorageValue<CanvasStore>["state"],
            onRehydrateStorage: () => () => {
                useCanvasStore.setState({ hydrated: true });
            },
        },
    ),
);

export function mergeCanvasProjects(
    remoteProjects: CanvasProject[],
    localProjects: CanvasProject[],
): CanvasProject[] {
    const projects = new Map<string, CanvasProject>();
    [...localProjects, ...remoteProjects].forEach((project) => {
        const previous = projects.get(project.id);
        if (
            !previous ||
            Date.parse(project.updatedAt || "") >=
            Date.parse(previous.updatedAt || "")
        ) {
            projects.set(project.id, project);
        }
    });
    return Array.from(projects.values()).sort(
        (a, b) =>
            Date.parse(b.updatedAt || "") -
            Date.parse(a.updatedAt || ""),
    );
}
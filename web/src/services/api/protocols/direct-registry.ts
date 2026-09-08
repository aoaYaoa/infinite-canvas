import type { DirectAIProvider } from "@/lib/model-channel";
import { apimartDirectProtocol } from "./apimart";
import { kieDirectProtocol } from "./kie";
import type { DirectProtocolAdapter } from "./types";

export const directProtocolAdapters: Readonly<Record<DirectAIProvider, DirectProtocolAdapter>> = {
    kie: kieDirectProtocol,
    apimart: apimartDirectProtocol,
};

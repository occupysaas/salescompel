export interface CandidateExchange {
  method: string;
  path: string;
  request: { contentType: string | null; body?: Uint8Array };
  response: { status: number; contentType: string | null; body: Uint8Array };
}
export type CandidateTrafficIssueCode =
  | "route"
  | "method"
  | "parameters"
  | "request-media"
  | "request-json"
  | "request-schema"
  | "status"
  | "response-media"
  | "response-json"
  | "response-schema"
  | "response-body"
  | "budget";
export interface CandidateTrafficResult {
  operationId?: string;
  requestIssues: CandidateTrafficIssueCode[];
  responseIssues: CandidateTrafficIssueCode[];
}
export interface CandidateTrafficValidator {
  validate(exchange: CandidateExchange): CandidateTrafficResult;
}

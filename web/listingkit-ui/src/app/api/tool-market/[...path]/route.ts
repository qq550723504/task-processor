import {
  handleToolMarket,
  rejectToolMarket,
} from "@/lib/server/tool-market-route";
export const dynamic = "force-dynamic";
export const GET = handleToolMarket;
export const POST = handleToolMarket;
export const PUT = handleToolMarket;
export const DELETE = rejectToolMarket;
export const PATCH = rejectToolMarket;
export const HEAD = rejectToolMarket;

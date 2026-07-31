import { NextResponse } from "next/server";

export const runtime = "nodejs";

const MAX_MESSAGE_LENGTH = 1_000;
const MAX_TEXT_LENGTH = 160;

function text(value: unknown, maxLength = MAX_TEXT_LENGTH) {
  return typeof value === "string" ? value.trim().slice(0, maxLength) : "";
}

function list(value: unknown) {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string")
    .map((item) => item.trim().slice(0, 40))
    .filter(Boolean)
    .slice(0, 8);
}

export async function POST(request: Request) {
  const webhook = process.env.LISTINGKIT_DEMO_WEBHOOK_URL?.trim();
  if (!webhook) {
    return NextResponse.json({ error: "Demo request delivery is not configured." }, { status: 503 });
  }

  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ error: "Invalid request." }, { status: 400 });
  }

  if (!payload || typeof payload !== "object") {
    return NextResponse.json({ error: "Invalid request." }, { status: 400 });
  }

  const input = payload as Record<string, unknown>;
  const name = text(input.name, 80);
  const company = text(input.company, 120);
  const contact = text(input.contact, 160);
  const platforms = list(input.platforms);
  const message = text(input.message, MAX_MESSAGE_LENGTH);

  if (!name || !company || !contact || platforms.length === 0) {
    return NextResponse.json({ error: "Please complete the required fields." }, { status: 400 });
  }

  const content = [
    "【ListingKit 预约演示】",
    `联系人：${name}`,
    `公司/店铺：${company}`,
    `联系方式：${contact}`,
    `目标渠道：${platforms.join("、")}`,
    `需求说明：${message || "未填写"}`,
  ].join("\n");

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch(webhook, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ msgtype: "text", text: { content } }),
      signal: controller.signal,
    });
    const result = await response.json().catch(() => null) as { errcode?: number } | null;
    if (!response.ok || result?.errcode !== 0) {
      console.error("Demo request webhook rejected the request.", { status: response.status, errcode: result?.errcode });
      return NextResponse.json({ error: "Unable to submit your request. Please try again." }, { status: 502 });
    }
  } catch (error) {
    console.error("Demo request webhook failed.", error);
    return NextResponse.json({ error: "Unable to submit your request. Please try again." }, { status: 502 });
  } finally {
    clearTimeout(timeout);
  }

  return NextResponse.json({ ok: true });
}

import client from './client';
import type { ApiResponse } from './auth';

// ============ Types ============

export type Rating = 'up' | 'down';

export type ReasonCode =
  | 'preference_matched' | 'helpful' | 'citation_reliable' | 'other'
  | 'citation_inaccurate' | 'requirement_misunderstood' | 'style_not_expected' | 'not_helpful';

export interface FeedbackResponse {
  rating: Rating;
  reason: ReasonCode;
  updated_at: string;
}

// Reason options grouped by rating for UI rendering.
export const UP_REASONS: { code: ReasonCode; label: string }[] = [
  { code: 'preference_matched', label: '回答符合我的偏好' },
  { code: 'helpful', label: '回答有帮助' },
  { code: 'citation_reliable', label: '引用清晰可信' },
  { code: 'other', label: '其他' },
];

export const DOWN_REASONS: { code: ReasonCode; label: string }[] = [
  { code: 'citation_inaccurate', label: '引用不准确' },
  { code: 'requirement_misunderstood', label: '记错了我的要求' },
  { code: 'style_not_expected', label: '风格不符合预期' },
  { code: 'not_helpful', label: '没有解决我的问题' },
  { code: 'other', label: '其他' },
];

// ============ API Functions ============

/**
 * Create or replace the current user's feedback for a message.
 */
export async function upsertFeedback(
  messageId: number,
  rating: Rating,
  reason: ReasonCode
): Promise<ApiResponse<FeedbackResponse>> {
  const res = await client.put<ApiResponse<FeedbackResponse>>(
    `/chat/messages/${messageId}/feedback`,
    { rating, reason }
  );
  return res.data;
}

/**
 * Delete the current user's feedback for a message.
 */
export async function deleteFeedback(
  messageId: number
): Promise<ApiResponse> {
  const res = await client.delete<ApiResponse>(
    `/chat/messages/${messageId}/feedback`
  );
  return res.data;
}

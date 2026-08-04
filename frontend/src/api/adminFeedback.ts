import client from './client';

// ============ Types ============

export interface FeedbackOverviewData {
  total_count: number;
  up_count: number;
  down_count: number;
  positive_ratio: number;
  reason_counts: Record<string, number>;
}

export interface AdminFeedbackItem {
  created_at: string;
  rating: string;
  reason: string;
}

export interface PageResponse<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
  total_page: number;
}

// ============ API Functions ============

/**
 * Get feedback overview statistics.
 */
export async function getFeedbackOverview(params: {
  from: string;
  to: string;
  rating?: string;
  reason?: string;
}): Promise<{ code: number; data: FeedbackOverviewData; message?: string }> {
  const res = await client.get('/admin/feedback/overview', { params });
  return res.data;
}

/**
 * Get paginated feedback list.
 */
export async function listFeedback(params: {
  from: string;
  to: string;
  rating?: string;
  reason?: string;
  page?: number;
  size?: number;
}): Promise<{ code: number; data: PageResponse<AdminFeedbackItem>; message?: string }> {
  const res = await client.get('/admin/feedback', { params });
  return res.data;
}

/**
 * Export feedback as CSV. Returns the raw Response for blob handling.
 */
export async function exportFeedbackCSV(params: {
  from: string;
  to: string;
  rating?: string;
  reason?: string;
}): Promise<Response> {
  const token = sessionStorage.getItem('access_token') || '';
  const query = new URLSearchParams();
  query.set('from', params.from);
  query.set('to', params.to);
  if (params.rating) query.set('rating', params.rating);
  if (params.reason) query.set('reason', params.reason);

  return fetch(`/api/v1/admin/feedback/export.csv?${query.toString()}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

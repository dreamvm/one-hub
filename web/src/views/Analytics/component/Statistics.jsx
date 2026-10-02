import { useState, useEffect } from 'react';
import { Alert, Grid } from '@mui/material';
import DataCard from 'ui-component/cards/DataCard';
import { gridSpacing } from 'store/constant';
import { showError, renderQuota } from 'utils/common';
import { API } from 'utils/api';
import { useTranslation } from 'react-i18next';

export default function Overview() {
  const { t } = useTranslation();
  const [loadError, setLoadError] = useState(false);
  const [userLoading, setUserLoading] = useState(true);
  const [channelLoading, setChannelLoading] = useState(true);
  const [rechargeLoading, setRechargeLoading] = useState(true);
  const [userStatistics, setUserStatistics] = useState({});

  const [channelStatistics, setChannelStatistics] = useState({
    active: 0,
    disabled: 0,
    test_disabled: 0,
    total: 0
  });

  const [rechargeStatistics, setRechargeStatistics] = useState({
    total: 0,
    Redemption: 0,
    Oder: 0,
    OderContent: ''
  });

  useEffect(() => {
    let active = true;
    const loadStatistics = async () => {
      try {
        const res = await API.get('/api/analytics/statistics');
        if (!active) return;
        const { success, message, data } = res.data;
        if (!success) {
          setLoadError(true);
          showError(message);
          return;
        }

        const userData = data.user_statistics || {};
        setUserStatistics({
          ...userData,
          total_quota: renderQuota(userData.total_quota || 0),
          total_used_quota: renderQuota(userData.total_used_quota || 0),
          total_direct_user: (userData.total_user || 0) - (userData.total_inviter_user || 0)
        });

        const channelData = { active: 0, disabled: 0, test_disabled: 0, total: 0 };
        (data.channel_statistics || []).forEach((item) => {
          if (item.status === 1) channelData.active = item.total_channels;
          else if (item.status === 2) channelData.disabled = item.total_channels;
          else if (item.status === 3) channelData.test_disabled = item.total_channels;
          channelData.total += item.total_channels;
        });
        setChannelStatistics(channelData);

        const redemption = (data.redemption_statistic || []).reduce((total, item) => total + item.quota, 0);
        let order = 0;
        const currencies = new Map();
        (data.order_statistics || []).forEach((item) => {
          order += item.quota;
          currencies.set(item.order_currency, (currencies.get(item.order_currency) || 0) + item.money);
        });
        setRechargeStatistics({
          total: renderQuota(redemption + order),
          Redemption: renderQuota(redemption),
          Oder: renderQuota(order),
          OderContent: [...currencies].map(([currency, money]) => `${currency}: ${money}`).join(' ')
        });
      } catch (error) {
        if (active) setLoadError(true);
      } finally {
        if (active) {
          setUserLoading(false);
          setChannelLoading(false);
          setRechargeLoading(false);
        }
      }
    };

    loadStatistics();
    return () => {
      active = false;
    };
  }, []);

  if (loadError) return <Alert severity="error">{t('common.unableServer')}</Alert>;

  return (
    <Grid container spacing={gridSpacing}>
      <Grid item lg={3} xs={12}>
        <DataCard
          isLoading={userLoading}
          title={t('analytics_index.totalUserSpending')}
          content={userStatistics?.total_used_quota || '0'}
          subContent={t('analytics_index.totalUserBalance') + '：' + (userStatistics?.total_quota || '0')}
        />
      </Grid>
      <Grid item lg={3} xs={12}>
        <DataCard
          isLoading={userLoading}
          title={t('analytics_index.totalUsers')}
          content={userStatistics?.total_user || '0'}
          subContent={
            <>
              {t('analytics_index.directRegistration')}：{userStatistics?.total_direct_user || '0'} <br />
              {t('analytics_index.invitationRegistration')}：{userStatistics?.total_inviter_user || '0'}
            </>
          }
        />
      </Grid>
      <Grid item lg={3} xs={12}>
        <DataCard
          isLoading={channelLoading}
          title={t('analytics_index.channelCount')}
          content={channelStatistics.total}
          subContent={
            <>
              {t('analytics_index.active')}：{channelStatistics.active} / {t('analytics_index.disabled')}：{channelStatistics.disabled}{' '}
              <br />
              {t('analytics_index.testDisabled')}：{channelStatistics.test_disabled}
            </>
          }
        />
      </Grid>
      <Grid item lg={3} xs={12}>
        <DataCard
          isLoading={rechargeLoading}
          title={'充值统计'}
          content={rechargeStatistics.total}
          subContent={
            <>
              兑换码: {rechargeStatistics.Redemption} <br /> 订单: {rechargeStatistics.Oder} / {rechargeStatistics.OderContent}
            </>
          }
        />
      </Grid>
    </Grid>
  );
}

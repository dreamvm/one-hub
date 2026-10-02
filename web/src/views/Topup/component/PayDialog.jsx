import PropTypes from 'prop-types';
import { useState, useEffect, useRef } from 'react';
import { Button, Dialog, DialogContent, DialogTitle, IconButton, Stack, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { useTheme } from '@mui/material/styles';
import { QRCode } from 'react-qrcode-logo';
import successSvg from 'assets/images/success.svg';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { useSelector } from 'react-redux';

const PayDialog = ({ open, onClose, amount, uuid }) => {
  const theme = useTheme();
  const siteInfo = useSelector((state) => state.siteInfo);
  const defaultLogo = theme.palette.mode === 'light' ? '/logo-loading.svg' : '/logo-loading-white.svg';
  const [message, setMessage] = useState('正在拉起支付中...');
  const [subMessage, setSubMessage] = useState(null);
  const [pollError, setPollError] = useState(null);
  const [loading, setLoading] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState(null);
  const [success, setSuccess] = useState(false);
  const onCloseRef = useRef(onClose);
  const cancelRef = useRef(() => {});
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  let useLogo = siteInfo.logo ? siteInfo.logo : defaultLogo;

  const clearValue = () => {
    setMessage('正在拉起支付中...');
    setSubMessage(null);
    setPollError(null);
    setLoading(false);
    setQrCodeUrl(null);
    setSuccess(false);
  };

  function openPayUrl(method, url, params) {
    const form = document.createElement('form');
    form.method = method;
    form.action = url;
    form.target = '_blank';
    for (const key in params) {
      const input = document.createElement('input');
      input.name = key;
      input.value = params[key];
      form.appendChild(input);
    }
    document.body.appendChild(form);
    form.submit();
    document.body.removeChild(form);
  }

  useEffect(() => {
    if (!open) {
      return;
    }
    let active = true;
    let timer;
    const cancel = () => {
      active = false;
      clearTimeout(timer);
    };
    cancelRef.current = cancel;
    clearValue();
    setLoading(true);

    const pollOrderStatus = (tradeNo) => {
      timer = setTimeout(async () => {
        try {
          const response = await API.get(`/api/user/order/status?trade_no=${encodeURIComponent(tradeNo)}`);
          if (!active) return;
          if (response.data.success) {
            setMessage('支付成功');
            setSubMessage(null);
            setPollError(null);
            setLoading(false);
            setSuccess(true);
            setQrCodeUrl(null);
            return;
          }
          setPollError(null);
        } catch {
          if (!active) return;
          setPollError('暂时无法查询支付状态，正在重试，请勿重复支付。');
        }
        if (active) pollOrderStatus(tradeNo);
      }, 3000);
    };

    const createOrder = async () => {
      try {
        const response = await API.post('/api/user/order', {
          uuid: uuid,
          amount: Number(amount)
        });
        if (!active) return;
        if (!response.data.success) {
          showError(response.data.message);
          setLoading(false);
          onCloseRef.current();
          return;
        }

        const { type, data } = response.data.data;
        if (type === 1) {
          setMessage('等待支付中...');
          setSubMessage(
            <>
              如果没有自动跳转，请点击
              <a
                href="#"
                onClick={(event) => {
                  event.preventDefault();
                  if (active) openPayUrl(data.method, data.url, data.params);
                }}
              >
                这里跳转
              </a>
            </>
          );
          openPayUrl(data.method, data.url, data.params);
        } else if (type === 2) {
          setQrCodeUrl(data.url);
          setLoading(false);
          setMessage('请扫码支付');
        }
        pollOrderStatus(response.data.data.trade_no);
      } catch {
        if (!active) return;
        setLoading(false);
        setMessage('无法确认订单创建结果');
        setSubMessage('请先核对订单或付款记录，再决定是否重试。');
      }
    };
    createOrder();
    return cancel;
  }, [open, amount, uuid]);

  //打开支付宝
  const handleOpenAlipay = (alipayUrl) => {
    if (alipayUrl && alipayUrl.startsWith('https://qr.alipay.com')) {
      window.open(alipayUrl, '_blank');
    }
  };
  return (
    <Dialog open={open} fullWidth maxWidth={'sm'} disableEscapeKeyDown>
      <DialogTitle sx={{ margin: '0px', fontWeight: 700, lineHeight: '1.55556', padding: '24px', fontSize: '1.125rem' }}>支付</DialogTitle>
      <IconButton
        aria-label="close"
        onClick={() => {
          cancelRef.current();
          clearValue();
          onClose();
        }}
        sx={{
          position: 'absolute',
          right: 8,
          top: 8,
          color: (theme) => theme.palette.grey[500]
        }}
      >
        <CloseIcon />
      </IconButton>
      <DialogContent>
        <DialogContent>
          <Stack direction="column" justifyContent="center" alignItems="center" spacing={2}>
            {loading && <img src={useLogo} alt="loading" height="100" />}
            {qrCodeUrl && (
              <QRCode
                value={qrCodeUrl}
                size={256}
                qrStyle="dots"
                eyeRadius={20}
                fgColor={theme.palette.primary.main}
                bgColor={theme.palette.background.paper}
              />
            )}
            {success && <img src={successSvg} alt="success" height="100" />}
            <Typography variant="h3">{message}</Typography>
            {pollError && <Typography role="status">{pollError}</Typography>}
            {subMessage && <Typography variant="body">{subMessage}</Typography>}
            {qrCodeUrl && qrCodeUrl.startsWith('https://qr.alipay.com') && !success && (
              <Button variant="contained" color="primary" onClick={() => handleOpenAlipay(qrCodeUrl)}>
                打开支付宝
              </Button>
            )}
          </Stack>
        </DialogContent>
      </DialogContent>
    </Dialog>
  );
};

export default PayDialog;

PayDialog.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  amount: PropTypes.number,
  uuid: PropTypes.string
};

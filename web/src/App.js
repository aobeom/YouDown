import React, { Component } from 'react';
import './App.css';

import PropTypes from 'prop-types'
import { withStyles } from '@material-ui/core/styles'
import classNames from 'classnames'

import Button from '@material-ui/core/Button';
import TextField from '@material-ui/core/TextField';
import CircularProgress from '@material-ui/core/CircularProgress';
import Snackbar from '@material-ui/core/Snackbar';


const styles = ({
    buttonProgress: {
        position: 'absolute',
        top: '50%',
        left: '50%',
        marginTop: -8,
        marginLeft: -12,
    },
    snackbarContent: {
        backgroundColor: "#ff0051",
    },
    customBtn: {
        color: "#fff",
        backgroundColor: "#d585e3",
        fontSize: "0.85rem",
        margin: "5px",
        '&:hover': {
            backgroundColor: "#9c27b0",
        },
    },
})

class App extends Component {
    constructor(props) {
        super(props)
        this.state = {
            durl: "",
            loading: false,
            btnText: "Go",
            message: "",
            open: false,
            btndis: {
                display: "none"
            },
        }
    }
    videoGet() {
        let targetURL = this.targetURL.value
        const you2be_reg = /youtube.com|youtu.be/
        const you2be_get = you2be_reg.exec(targetURL)
        if (you2be_get !== null) {
            const you2be_site = you2be_get[0]
            if (you2be_site === "youtube.com") {
                const v_uri = targetURL.split("?")[0]
                const v_reg = /v=[\w]+/
                const para_v = v_reg.exec(targetURL)[0]
                targetURL = v_uri + "?" + para_v
            } else {
                const v_uri = targetURL.split("/")[3]
                targetURL = "https://www.youtube.com/watch?v=" + v_uri
            }
        } else {
            this.setState({
                status: 1,
                message: "URL ERROR",
                open: true,
            })
            return false
        }
        const apiurl = "//yourapiaddr/api/y2b"
        this.setState({
            loading: true,
            values: "",
            btnText: "Loading...",
            btndis: {
                display: "none"
            }
        })
        fetch(apiurl, {
            method: 'POST',
            dataType: 'json',
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({ url: this.targetURL.value })
        }).then(res => res.json())
            .then(data => {
                let status = data.status
                if (status === 0) {
                    let download_url = data.download_url
                    let video_title = data.title
                    this.setState({
                        status: 0,
                        btndis: {
                            display: "block"
                        },
                        durl: download_url,
                        dtitle: video_title,
                        loading: false,
                        btnText: "Go"
                    })
                } else {
                    this.setState({
                        status: 1,
                        loading: false,
                        message: data.message,
                        open: true,
                        btnText: "Go"
                    })
                }
            })
            .catch(
                () => this.setState({
                    status: 1,
                    loading: false,
                    open: true,
                    message: "Server Error",
                    btnText: "Go"
                })
            )
    }
    tipClose() {
        this.setState({
            open: false,
        })
    }
    render() {
        const { classes } = this.props
        const status = this.state.status
        const durl = this.state.durl
        const { loading } = this.state
        const btnText = this.state.btnText
        const vTmp = []
        if (status === 0) {
            vTmp.push(
                <a href={durl} target="_blank" download rel="noopener noreferrer" style={this.state.btndis}>
                    <Button variant="contained" className={classNames(classes.customBtn)}>{this.state.dtitle}</Button>
                </a>
            )
        }
        return (
            <div className="App">
                <div className="AppWrapper">
                    <div className="AppItem">
                        <TextField
                            id="standard-name"
                            label="Youtube URL"
                            inputRef={u => this.targetURL = u}
                        />
                    </div>
                    <div className="AppItem">
                        <Button disabled={loading} variant="contained" color="primary" onClick={this.videoGet.bind(this)}>
                            {btnText}
                        </Button>
                        {loading && <CircularProgress size={24} className={classes.buttonProgress} />}
                    </div>
                </div>
                <div className="AppData">
                    {vTmp}
                </div>
                <Snackbar
                    anchorOrigin={{ vertical: 'top', horizontal: 'center' }}
                    autoHideDuration={3000}
                    onClose={this.tipClose.bind(this)}
                    open={this.state.open}
                    ContentProps={{
                        'aria-describedby': 'message-id',
                        className: classes.snackbarContent,
                    }}
                    message={<span id="message-id">{this.state.message}</span>}
                />
            </div>
        );
    }
}

App.propTypes = {
    classes: PropTypes.object.isRequired,
}

export default withStyles(styles)(App)

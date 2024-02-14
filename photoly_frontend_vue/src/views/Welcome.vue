<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";

</script>

<template>
<div id="main">
    <div id="photoDiv">
        <div id="barDiv">
            <div id="logoDiv">
                <img id="logoImg" src="@/assets/logo.ico"/>
                <p id="siteNoteP">PHOTOLY</p>
            </div>
            <div id="barOptionContainerDiv">
                <n-button quaternary round size="large" style="color: aliceblue;" @click="toSignup">
                    Sign Up
                </n-button>
                <n-button quaternary round size="large" style="color: aliceblue;" @click="toLogin">
                    Sign In
                </n-button>
            </div>
        </div>
        <div id="welcomeTextDiv">
            <p class="welcomeText">
                See the Value of Photo Inside
            </p>
            <p class="welcomeText">
                The Beauty Shines Here
            </p>
            <p class="welcomeTextSub">
                Photoly - Share Your Inspiration
            </p>
            <div id="welcomeTextDivSub1">
                <button class="welcomeButton" onclick="" type="button" @click="toSignup">
                    Sign Up
                </button>
                <button class="welcomeButton" onclick="" type="button" @click="toLogin">
                    Sign In
                </button>
            </div>
        </div>
    </div>

    <div id="nextDiv">
        <p class="headingsP">Project Summary</p>
        <div class="projectSummaryDiv">
            <p>
                There are demanding for self-host and reliable picture host application. Photoly provides a solution just right for that. Using Photoly, you can manage your pictures and photos with easy clicks, categorize them using tags, galleries, and namespaces. Moreover, Photoly provides API for you to access our functionalities for embedding in other applications.
            </p>
        </div>
        <div class="projectSummaryDiv">
            <p class="projectSummaryP">
                <a class="projectSummaryA" href="">Star our GitHub Repo! <img class="smallIcons" src="@/assets/icons/ExternalLink.svg"/></a>
            </p>
        </div>
    </div>

    <div id="endDiv">
        <p class="headingsP">About Us</p>
        <div class="projectSummaryDiv">
            <div class="aboutUsImgDiv">
                <img class="aboutUsImg" src="https://avatars.githubusercontent.com/u/108884969"/>
            </div>
            <div class="aboutUsIntroDiv">
                <p class="aboutUsP">
                    <a class="aboutUsA" href="https://github.com/WiscWebGroup">WiscWebGroup <img class="smallIcons" src="@/assets/icons/ExternalLink.svg"/></a>
                </p>
                <p class="aboutUsPSub">
                    We are WiscWebGroup, a team located in Madison, Wisconsin. We mainly develop interesting web applications.
                </p>
            </div>
            
        </div>
        <div class="projectSummaryDiv">
            <div class="aboutUsImgDiv">
                <img class="aboutUsImg" src="https://avatars.githubusercontent.com/u/68500948"/>
            </div>
            <div class="aboutUsIntroDiv">
                <p class="aboutUsP">
                    <a class="aboutUsA" href="https://github.com/Harold-y">Yixuan Ye (Harold-y) <img class="smallIcons" src="@/assets/icons/ExternalLink.svg"/></a>
                </p>
                <p class="aboutUsPSub">
                    BS from UW-Madison. I did the DB, backend, CI/CD, part of the frontend work, and also designed the logo.
                </p>
            </div>
            
        </div>
        <div class="projectSummaryDiv">
            <div class="aboutUsImgDiv">
                <img class="aboutUsImg" src="https://avatars.githubusercontent.com/u/31459252"/>
            </div>
            <div class="aboutUsIntroDiv">
                <p class="aboutUsP">
                    <a class="aboutUsA" href="https://github.com/Slijeff">Jeffrey Hui (Slijeff) <img class="smallIcons" src="@/assets/icons/ExternalLink.svg"/></a>
                </p>
                <p class="aboutUsPSub">
                    BS from UW-Madison. I did part of the frontend job in this project. I photographed the background images for the home page.
                </p>
            </div>
            
        </div>
        <div class="projectSummaryDiv">
            <div class="aboutUsImgDiv">
                <img class="aboutUsImg" src="https://avatars.githubusercontent.com/u/46753484"/>
            </div>
            <div class="aboutUsIntroDiv">
                <p class="aboutUsP">
                    <a class="aboutUsA" href="https://github.com/NataRich">Zihan Zhao (NataRich) <img class="smallIcons" src="@/assets/icons/ExternalLink.svg"/></a>
                </p>
                <p class="aboutUsPSub">
                    Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
                </p>
            </div>
            
        </div>
    </div>
    
        
</div>
</template>

<style scoped>
    #main {
        padding: 0;
        margin: 0;
        display: block;
    }
    #photoDiv {
        display:flex;
        align-items: center;
        -webkit-box-pack: center;
        justify-content: center;
        height: 100vh;
        background-size: cover;
        position: relative;
        background-attachment: fixed;
        background-position: center 0;
        background-repeat: no-repeat;
        background-image: url("@/assets/JH_20211030_5406.jpg");
        border-bottom: 2px solid #cccccc
    }
    #barDiv {
        display: flex;
        -webkit-box-align: center;
        align-items: center;
        -webkit-box-pack: justify;
        justify-content: space-between;
        flex-direction: row;
        background: rgba(0, 0, 0, 0.18) ;
        width: 100%;
        height: 2.05rem;
        color: #FFFFFF;
        padding: 1rem;
        position: fixed;
        top: 0px;
    }
    #logoDiv {
        display: flex;
        -webkit-box-align: center;
        align-items: center;
        flex-direction: row;
        cursor: default;
        margin-left: 1.25rem;
    }
    #logoImg {
        display: block;
        width: 50px;
        max-width: 100%;
    }
    .smallIcons {
        display: inline;
        width: 15px;
        max-width: 100%;
    }
    #barOptionContainerDiv {
        display: flex;
        -webkit-box-align: center;
        align-items: center;
        flex-direction: row;
    }
    #siteNoteP {
        font-size: 1.2rem;
        font-weight: 520;
        margin-inline-start: 0.86rem;
    }
    #welcomeTextDiv {
        display: flex;
        -webkit-box-align: center;
        align-items: center;
        flex-direction: column;
    }
    .welcomeText {
        font-size: 3.3rem;
        color: #FFFFFF;
        font-weight: 400;
        margin: 0;
        margin-bottom: 1rem;
        font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    }
    .welcomeTextSub {
        font-size: 1.5rem;
        color: #B2F5EA;
        font-weight: 400;
        margin: 0;
        margin-bottom: 1rem;
        font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    }
    .welcomeButton {
        align-items: center;
        justify-content: center;
        border-radius: 0.375rem;
        font-weight: 550;
        height: 2.5rem;
        width: 5rem;
        background: rgba(200, 245, 245, 0.35);
        color: #FEFEFE;
        border: 3px solid rgba(117, 166, 250, 0.4);
        transition-duration: 0.75s; 
        margin: 1.25rem;
    }
    .welcomeButton:hover {
        background: rgba(255, 255, 255, 0.20);
        border: 3px solid rgba(117, 166, 250, 0.1);
        color: #EEEEEE;
        transition-duration: 0.75s;
        cursor: pointer;
    }
    #nextDiv {
        height: 100vh;
        display:flex;
        align-items: center;
        -webkit-box-pack: center;
        background-size: cover;
        position: relative;
        background-attachment: fixed;
        background-position: center 0;
        background-repeat: no-repeat;
        background-image: url("@/assets/JH_20220710_5856.jpg");
        flex-direction: column;
    }
    .headingsP {
        margin-top: 8rem;
        font-size: 2.75rem;
        color: #fbfbfb;
        font-weight: 600;
        font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    }
    .projectSummaryDiv {
        display: flex;
        width: 28vw;
        display: flex;
        flex-direction: row;
        align-items: center;
        background: rgba(255, 255, 255, 0.76);
        border-radius: 1.75rem;
        padding-left: 2.5rem;
        padding-right: 2.5rem;
        padding-top: 0.4rem;
        padding-bottom: 0.4rem;
        color: rgba(29, 64, 68, 0.75);
        line-height: 1.6;
        margin-bottom: 2rem;
        font-size: 15px;
    }
    .projectSummaryP {
        display: block;
        margin: 0 auto;
        padding: 0.6rem;

    }
    .projectSummaryA {
        color: rgba(29, 64, 68, 0.75);
        line-height: 1.6;
        text-decoration: none;
    }
    #endDiv {

        display:flex;
        align-items: center;
        -webkit-box-pack: center;
        background-size: cover;
        position: relative;
        background-attachment: fixed;
        background-position: center 0;
        background-repeat: no-repeat;
        background-image: url("@/assets/JH_20220601_5686.jpg");
        flex-direction: column;
    }
    .aboutUsImgDiv {
        display:flex;
        flex-direction: column;
        align-items: center;
        -webkit-box-pack: center;
    }
    .aboutUsIntroDiv {
        margin-left: 2rem;
    }
    .aboutUsImg {
        display: block;
        width: 15rem;
        max-width: 100%;
        border-radius: 5rem;
    }
    .aboutUsP {
        display: block;
        margin: 0 auto;
        padding-top: 0.6rem;
    }
    .aboutUsA {
        font-size: 1.4rem;
        line-height: 1.6;
        text-decoration: none;
        color: #000000;
    }
    .aboutUsPSub {
        line-height: 1.6;
        color: rgba(0, 0, 0, 0.68)
    }
</style>

<script>
function toLogin() {
    router.push('/login');
}
function toSignup() {
    router.push('/signup');
}
</script>
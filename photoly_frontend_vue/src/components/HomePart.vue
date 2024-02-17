<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import {Folder28Regular, Search20Regular} from "@vicons/fluent"
import { ref, defineComponent, nextTick } from 'vue';
import { NTag } from "naive-ui";
import { useMessage } from 'naive-ui'
</script>

<template>
  <div>
    <n-back-top :right="100" />
    <n-breadcrumb separator=">">
      <n-icon size="25" :depth="3" style="vertical-align:top">
        <svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24"><g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M5 7l5 5l-5 5"></path><path d="M13 17h6"></path></g>
        </svg>
      </n-icon>
      <n-breadcrumb-item v-for="pathItem in nsPath" v-on:click="setCurrNsId(pathItem.nsId)">
        <n-icon /> {{ pathItem.nsName }}</n-breadcrumb-item>
    </n-breadcrumb>

    <n-divider />
    <div @contextmenu="blankSpaceMenu">
      <div>
        <n-h2>Folders</n-h2>
        <n-space>
          <n-button secondary strong size="large" v-for="folder in nsChildren" v-on:click="setCurrNsId(folder.nsId)" @click.right.native="folderSpaceMenu">
            <template #icon>
              <n-icon>
                <Folder28Regular />
              </n-icon>
            </template>
            {{ folder.nsName.length <= 14 ? folder.nsName : folder.nsName.substring(0, 12) + "..." }}
          </n-button>
        </n-space>
      </div>

      <div style="margin-top: 1rem;">
        <n-h2>Medias</n-h2>
        <n-space>
          <n-card style="border-radius: 20px;" v-for="photo in photoChildren" v-on:click="photoModalOpen(photo)" hoverable @click.right.native="photoSpaceMenu">
            <template #cover>
              <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 1"><img v-bind:src="baseUThumbnail + userToken + '?photoId=' + photo.photoId" class="cardImg"></div>
              <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 2"><img src="@/assets/icons/Video.png" class="cardImg"></div>
              <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === -1"><img src="@/assets/logo.ico" class="cardImg"></div>
            </template>
            <p>{{ photo.photoName.length <= 14 ? photo.photoName : photo.photoName.substring(0, 12) + "..." }}.{{ photo.format }}</p>
          </n-card>
        </n-space>
      </div>
      <n-modal v-model:show="showPhoto">
          <n-card
          style="width: 85vw; padding: 0; border-radius: 0.375rem;"
            :bordered="false"
            size="huge"
            role="dialog"
            aria-modal="true"
          >
            <div style="display: flex; align-items: flex-start; flex-direction: row;">
              <img v-show="isphotoOrVideo(showPhotoInfo.format) === 1" v-bind:src="baseUPhoto + userToken + '?photoId=' + showPhotoInfo.photoId" 
              style="object-fit: fill; border-radius: 0.375rem; width: 75%; height: 100%; max-width: 90%;"/>
              <video
                  v-show="isphotoOrVideo(showPhotoInfo.format) === 2"
                  :key="baseUVideo"
                  width="75%"
                  controls
                >
                  <source
                    :src="baseUVideo"
                    :type="'video/' + showPhotoInfo.format"
                  >
              </video>

              <div style="margin-left: 2rem;">
                <n-h2 >Metadata</n-h2>
                
                <n-collapse :default-expanded-names="['cm']">
                  <n-collapse-item title="Common Metadata" name="cm">
                    <p>Name: {{ showPhotoInfo.photoName.length <= 14 ? showPhotoInfo.photoName : showPhotoInfo.photoName.substring(0, 12) + "..." }}.{{ showPhotoInfo.format }}</p>
                    <p>Upload Date: {{ showPhotoInfo.uploadDate }}</p>
                    <p>Format: {{ showPhotoInfo.format }}</p>
                    <p>Path: {{ nsPathStr }}</p>
                    <p>Shared: {{ showPhotoInfo.visibility === 0 ? "No" : "Yes" }}</p>
                    <n-button strong secondary type="success" @click="download">
                      Download
                    </n-button>
                  </n-collapse-item>
                  <n-collapse-item title="Secrets" name="se">
                    <p>Token: {{ showPhotoInfo.token }}</p>
                    <p>UUID: {{ showPhotoInfo.photoUuid }}</p>
                  </n-collapse-item>
                  <n-collapse-item title="Tags" name="tag">
                    <n-tag type="success" size="small" round closable @close="removeTag(tagItem.tag_id)" v-for="tagItem in showPhotoTagList">
                      {{ tagItem.tag_name }}
                      
                    </n-tag>

                    <n-divider />
                    <p>Add</p>
                    <!--
                      <n-input-group>
                      <n-input :style="{ width: '100px' }" />
                      <n-button type="primary" ghost style="width: 50px;">
                        <template #icon>
                          <n-icon><Search20Regular/></n-icon>
                        </template>
                      </n-button>
                    </n-input-group>
                    -->
                    <n-select v-model:value="tagVal" :options="tagOptions" v-on:click="getAllTags"/>  
                    <n-button circle style="margin-top: 5px;" @click="addTag">
                      Submit
                    </n-button>
                  </n-collapse-item>
                  <n-collapse-item title="Gallery" name="ga">
                    <n-tag type="success" size="small" round closable @close="removeGa(GaItem.ga_id)" v-for="GaItem in showPhotoGaList">
                      {{ GaItem.ga_name }}
                      
                    </n-tag>
                    <n-divider />
                    <p>Add</p>
                    <!--
                      <n-input-group>
                      <n-input :style="{ width: '100px' }" />
                      <n-button type="primary" ghost style="width: 50px;">
                        <template #icon>
                          <n-icon><Search20Regular/></n-icon>
                        </template>
                      </n-button>
                    </n-input-group>
                    -->

                    <n-select v-model:value="gaVal" :options="gaOptions" v-on:click="getAllGas"/>
                    <n-button circle style="margin-top: 5px;" @click="addGa">
                      Submit
                    </n-button>
                  </n-collapse-item>
                  <n-collapse-item title="Share" name="share">
                    <n-button strong secondary type="default" @click="copyShareAddr" style="width: 10rem;">
                      Copy Share Addr
                    </n-button>
                    <n-popover trigger="click">
                      <template #trigger>
                        <n-button strong secondary type="info" style="margin-top: 10px; width: 10rem">
                          Get QR Code
                        </n-button>
                      </template>
                      <img v-bind:src="baseQR + 'token=' + userToken + '&photoId=' + showPhotoInfo.photoId" style="width:170px;"/>
                    </n-popover>
                    <div style="width: 10rem; display: flex; flex-direction: row; align-items:center;">
                      <p>Visibility</p>
                      <n-switch v-model:value="showPhotoInfoVisibility" @update:value="changeVisibility" style="margin-left: 8px;"/>
                    </div>
                  </n-collapse-item>
                </n-collapse>
              </div>
            </div>
            

          </n-card>
      </n-modal>


      <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="blankSpaceMenuOptions"
        :show="showBlankSpaceMenu"
        :on-clickoutside="onClickOutside('blank')"
        @select="handleSelect"
      />
    </div>

  </div>
</template>

<script>

export default defineComponent({
  data() {
    return {
      nsId: -1,
      parentNsId: -1,
      nsName: "",
      nsChildren: [],
      photoChildren: [],
      nsPath: [],
      nsPathStr: "",
      baseUThumbnail: import.meta.env.VITE_APP_BASE_URL + "/photo/renderThumbnail/" ,
      baseUPhoto: import.meta.env.VITE_APP_BASE_URL + "/photo/render/" ,
      baseQR: import.meta.env.VITE_APP_BASE_URL + "/photo/getQR2?",
      userToken: localStorage.getItem("HRD-Token"),
      showPhoto: ref(false),
      showPhotoInfo: null,
      showPhotoInfoVisibility: false,
      showPhotoTagList: [],
      showPhotoGaList: [],
      tagVal: ref(null),
      gaVal: ref(null),
      tagOptions: [],
      gaOptions: [],
      baseUVideo: "" ,
      showBlankSpaceMenu: false,
      showFolderSpaceMenu: false,
      showPhotoSpaceMenu: false,
      mouseX: null,
      mouseY: null,
      blankSpaceMenuOptions: [
        {
          label: "Upload Photo",
          key: "upload"
        },
        {
          label: "Create Folder",
          key: "create_folder"
        }
      ],
      FolderSpaceMenuOptions: [],
      PhotoSpaceMenuOptions: [],
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    blankSpaceMenu (e) {
      e.preventDefault();
      window.$message.success("blankSpaceMenu!")
      this.showBlankSpaceMenu = false;
      nextTick().then(() => {
        this.showBlankSpaceMenu = true;
        this.mouseX = e.clientX;
        this.mouseY = e.clientY;
        
        });
      
    },
    handleSelect(key) {
      this.showBlankSpaceMenu = false;
      window.$message.success(String(key))
    },
    onClickOutside (which) {
      if (which === "blank")
      {
        this.showBlankSpaceMenu = false;
      }
      if (which === "photo")
      {
        this.showPhotoSpaceMenu = false;
      }
      if (which === "folder")
      {
        this.showFolderSpaceMenu = false;
      }
    },
    folderSpaceMenu(e) {
      e.preventDefault();
      e.stopPropagation();
      window.$message.success("folderSpaceMenu!")
    },
    photoSpaceMenu(e) {
      e.preventDefault();
      e.stopPropagation();
      window.$message.success("photoSpaceMenu!")
    },
    isphotoOrVideo (format) {
      format = format.toLowerCase();
      let photoFormats = ["jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "svg", "psd", "raw", "arw", "pcd"]
      let videoFormats = ["mp4", "avi", "wmv", "mpeg", "m4v", "mov", "flv", "asf", "f4v", "rmvb", "vob", "rm"]
      if (photoFormats.includes(format))
      {
        return 1;
      }
      if (videoFormats.includes(format))
      {
        return 2;
      }
      return -1;
    },
    changeVisibility() {
      this.showPhotoInfoVisibility ? this.showPhotoInfo.visibility = 1 : this.showPhotoInfo.visibility = 0;
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/update",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
          "photoId": this.showPhotoInfo.photoId,
          "visibility": this.showPhotoInfoVisibility ? 1 : 0
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          navigator.clipboard.writeText(response.data.t);
          window.$message.success("visibility changed!")
        }else {
          window.$message.warning("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    copyShareAddr() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getPath?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          navigator.clipboard.writeText(response.data.t);
          window.$message.success("Successful copied to clipboard!")
        }else {
          window.$message.warning("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    download() {
      axios
          .get(import.meta.env.VITE_APP_BASE_URL + "/photo/download/"+ this.userToken +"?photoId=" + this.showPhotoInfo.photoId, {responseType: "blob"})
          .then(async (response) => {
            var a = document.createElement("a");
            document.body.appendChild(a);
            a.style = "display: none";

            let url = window.URL.createObjectURL(new Blob([response.data]));

            let link = document.createElement("a");
            link.style.display = "none";
            link.href = url;
            link.download = this.showPhotoInfo.photoName + "." + this.showPhotoInfo.format;
            document.body.appendChild(link);
            link.click();
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
    },
    addTag() {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/addTags?photoId=" + this.showPhotoInfo.photoId + "&tagIds=" + this.tagVal,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.tagVal = ref(null);
          this.getTags();
          window.$message.success("added to tag!")
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    addGa() {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/addToGallery?photoId=" + this.showPhotoInfo.photoId + "&gaId=" + this.gaVal,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.gaVal = ref(null);
          this.getGas();
          window.$message.success("added to gallery!")
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    getAllTags() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/tag/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let alltags = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var tag = infoT[i];
            alltags.push({
              "label": tag["tagName"], 
              "value": tag["tagId"]
            })
          }
          this.tagOptions = alltags;
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    getAllGas() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/gallery/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let allgas = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var gas = infoT[i];
            allgas.push({
              "label": gas["gaName"], 
              "value": gas["gaId"]
            })
          }
          this.gaOptions = allgas;
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    removeGa(gaId) {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteFromGallery?photoId=" + this.showPhotoInfo.photoId + "&gaId=" + gaId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.getGas();
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    removeTag(tagId) {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteTag?photoId=" + this.showPhotoInfo.photoId + "&tagId=" + tagId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.getTags();
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    getTags() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getTagByPhoto?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.showPhotoTagList = infoT;
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    getGas() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getGalleryByPhoto?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.showPhotoGaList = infoT;
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    photoModalOpen(photoInfo) {
      this.showPhotoInfo = photoInfo;
      this.showPhotoInfoVisibility = this.showPhotoInfo.visibility === 1 ? true : false;
      this.getTags();
      this.getGas();
      this.showPhoto = true;

      this.baseUVideo = import.meta.env.VITE_APP_BASE_URL + "/photo/renderV/" + this.userToken + "?photoId=" + this.showPhotoInfo.photoId
    },
    setCurrNsId(nsId) {
      if (this.nsId !== nsId)
      {
        this.nsId = nsId;
        this.update();
      }
      
    },
    trace() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/trace?nsId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.nsPath = infoT;
          this.nsPathStr = "";
          for(var i = 0; i < infoT.length; i ++)
          {
            if (i == 0)
            {
              this.nsPathStr += "/root";
            }else {
              this.nsPathStr += "/" + infoT[i]["nsName"];
            }
          }
          if (infoT.length < 2)
          {
            this.parentNsId = -1;
          }else {
            this.parentNsId = infoT[infoT.length - 2]["nsId"];
          }
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    queryNsChildren () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getChildren?parentId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.nsChildren = infoT
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    queryPhotos() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getByNamespace?nsId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.photoChildren = infoT
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    update() {
      this.queryPhotos();
      this.queryNsChildren();
      this.trace();
    },
    queryRoot() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getRoot/",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.nsId = infoT.nsId;
          this.nsName = infoT.nsName;
          this.update();
        }else {
          window.$message.error("User Info Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    }
  },
  computed: {

  },
  mounted () {
    if (this.parentNsId === -1)
    {
      this.queryRoot();
    }
    window.$message = useMessage()
  },
  components: {

  }
})

</script>

<style scoped>
.n-button {
  border-radius: 10px;
  width: 15rem;
  
}
.cardImgDiv {
  width: 255px;
  height: 128px;
}
.cardImg {
  object-fit: cover;
  width: 100%;
  max-width: 100%;
  height: 128px;
  overflow: hidden;
}
.n-card {
  /*cursor: pointer;*/
  padding: 0;
}
</style>

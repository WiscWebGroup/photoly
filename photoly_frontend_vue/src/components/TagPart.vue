<script setup>
import axios from "axios";
import {Archive16Regular as ArchiveIcon, FolderAdd20Regular, FolderArrowRight20Regular, ArrowUpload20Filled,
  ArrowDownload20Filled, Eye20Regular, Delete20Regular, Edit20Regular } from "@vicons/fluent"
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import MoveToShowFolder from "./MoveToShowFolder.vue"
</script>

<template>
  <div>

    <!-- This part is for medias -->

    <div @contextmenu="blankSpaceMenu">

      <!-- This part is for loading photos cards into the "media" section -->

      <div style="margin-top: 1rem;">
        <div style="display: flex; align-items: flex-start; flex-direction: row;">

          <div style="width: 15vw;">
            <n-h2>Tags</n-h2>
            <n-collapse :default-expanded-names="['show']">

              <n-collapse-item title="Add New Tag" name="add">
                <n-card title="" size="small" >
                      Add a New Tag
                      <template #footer>
                        <n-space vertical>
                          <n-input v-model:value="addNewTagInput" type="text" placeholder="Tag Name" @keyup.enter="createTag" />
                          <n-button strong secondary style="width: 100%; " @click="createTag">
                            Add
                          </n-button>
                        </n-space>
                      </template>
                </n-card>
              </n-collapse-item>

              <n-collapse-item title="Tags" name="show">
                
                <n-flex vertical>
                  <n-tag v-for="tag in tagChildren" @click.left.native="() => {
                    if (renderPhotoTagIds.includes(tag.tagId))
                    {
                      renderPhotoTagIds = renderPhotoTagIds.filter(tagIdT => tagIdT !== tag.tagId)
                    }else {
                      renderPhotoTagIds.push(tag.tagId)
                    }
                    if (renderPhotoTagIds.length > 0)
                    {
                      queryPhotos();
                    }else {
                      photoChildren = []
                    }
                    }"
                    :type="renderPhotoTagIds.includes(tag.tagId) ? 'success' : 'info'" @click.right.native="TagMenu(tag, $event)"
                    class="tagCard">
                      {{ tag.tagName > 15 ? tag.tagName.substring(0, 15) + "..." : tag.tagName }} 
                  </n-tag>
                </n-flex>
              </n-collapse-item>

            </n-collapse>
            
          </div>

          <div style="margin-left: 2rem;">
            <n-space style="align-items:baseline">
              <n-h2>Medias</n-h2>
              <n-switch v-model:value="photoMultiSelectMode" >
                <template #checked>
                  Photo Select Mode
                </template>
                <template #unchecked>
                  Select Mode Off
                </template>
              </n-switch>
              <n-button round style="width: 5rem" v-show="photoMultiSelectMode" @click="clearCheckboxPhoto">
                  Clear
              </n-button>
              <n-button round style="width: 7rem" v-show="photoMultiSelectMode" @click="selectAllCheckboxPhoto">
                  Select All
              </n-button>
              <n-button round style="width: 7rem" v-show="photoMultiSelectMode" @click="moveSelectedPhoto" type="info">
                  Move
              </n-button>
              <n-button round style="width: 12rem" v-show="photoMultiSelectMode" @click="showDetach" type="warning">
                  Remove From Tag
              </n-button>
              <n-button round style="width: 10rem" v-show="photoMultiSelectMode" @click="() => {showPhotoMultiDeleteModal = true;}" type="error">
                  Delete Selected
              </n-button>
            </n-space>
            <n-empty v-show="photoChildren.length <= 0" style="padding-left: 30vw; padding-top: 20vh;">
            </n-empty>
            <n-space>
              <div v-for="photo in photoChildren">
                <n-card style="border-radius: 20px; background-color: #f4f2f2;" v-show="photoMultiSelectMode">
                  <n-checkbox size="large"
                  style="margin-left: 0.5rem;" @update:checked="() => {photo.checked = !photo.checked}" :checked="photo.checked"/>
                </n-card>
                <n-card style="border-radius: 20px;" @click.left.native="handlePhotoMulti(photo, $event)" hoverable @click.right.native="photoSpaceMenu(photo, $event)">
                  <template #cover>
                    <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 1"><img v-bind:src="baseUThumbnail + userToken + '?photoId=' + photo.photoId" class="cardImg"></div>
                    <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 2"><img src="@/assets/icons/Video.png" class="cardImg"></div>
                    <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === -1"><img src="@/assets/logo.ico" class="cardImg"></div>
                  </template>
                  <p>{{ photo.photoName.length <= 14 ? photo.photoName : photo.photoName.substring(0, 12) + "..." }}.{{ photo.format }}</p>
                </n-card>
              </div>
            </n-space>
          </div>

        </div>
        
      </div>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

      <!-- This modal is for the photo detail after click on 'view' or open by left click the photo -->
      <n-modal v-model:show="showPhoto">
        <div style="min-width: 40vw ;max-width: 85vw; padding: 0; border-radius: 0.375rem;">
          <n-card
          style="padding: 0; border-radius: 0.375rem;"
            :bordered="false"
            size="huge"
            role="dialog"
            aria-modal="true"
          >
            <div style="display: flex; align-items: flex-start; flex-direction: row;">
              <img v-show="isphotoOrVideo(showPhotoInfo.format) === 1" v-bind:src="baseUPhoto + userToken + '?photoId=' + showPhotoInfo.photoId" 
              style="object-fit: fill; border-radius: 0.375rem; height: 100%; min-width: 30%; max-width: 75%;" @click.left.native="() => {showPhoto = false}"/>
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

              <div style="padding-left: 2rem;">
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
                  <n-collapse-item title="Gallery" name="ga" style="max-width: 100%;">
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
                    <n-space vertical :size=1>
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
                    </n-space>
                    
                  </n-collapse-item>

                  <n-collapse-item title="Edit Info" name="ei">
                    <p>Visibility</p>
                    <n-switch v-model:value="showPhotoInfoVisibility" @update:value="changeVisibility" style="margin-left: 8px;"/>
                    <p>Change Name</p>
                    <n-input-group>
                        <n-input type="text" round size="small" placeholder="New Name" v-model:value="newNameVal" @keyup.enter="changeName" />
                        <n-button type="primary" ghost style="width:5rem" @click="changeName">
                          Update
                        </n-button>
                      </n-input-group>
                  </n-collapse-item>

                </n-collapse>
              </div>
            </div>
            

          </n-card>
        </div>
      </n-modal>

      <!-- ENDOF: This modal is for the photo detail after click on 'view' or open by left click the photo -->

    </div>
    <!-- ENDOF: This part is for medias -->

    <!-- MENU PART -->
    <!-- This part is for redesign of right click menu for media -->
      <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="PhotoSpaceMenuOptions"
        :show="showPhotoSpaceMenu"
        :on-clickoutside="onClickOutsidePhoto"
        @select="handleSelectPhoto"
      />
    <!-- ENDOF: This part is for redesign of right click menu for media -->

    <!-- This part is for redesign of right click menu for tag part -->
    <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="TagMenuOptions"
        :show="showTagMenu"
        :on-clickoutside="onClickOutsideTag"
        @select="handleSelectTag"
      />
    <!-- ENDOF: This part is for redesign of right click menu for tag part -->
    <!-- ENDOF: MENU PART -->

    <!-- MODAL PART -->

    <!-- This part is for modal of moveTo Functions -->
    <MoveToShowFolder @moveToUpdate="moveItem" ref="moveToModuleRef" :originalNsId="moveToModalOriginalNsId" :disableOrgNsId="moveToModalDisableOrgNsId"/>
    <!-- ENDOF: This part is for modal of moveTo Functions -->

    <!-- This part is for modal of Multiple moveTo Functions -->
    <MoveToShowFolder @moveToUpdate="moveItemMulti" ref="moveToMultiModuleRef" :originalNsId="moveToModalOriginalNsId" :disableOrgNsId="moveToModalDisableOrgNsId"/>
    <!-- ENDOF: This part is for modal of Multiple moveTo Functions -->

    <!-- This part is for modal of confirming to delete a tag -->
    <n-modal v-model:show="showDeleteTagConfirmModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE this tag?
        <p>*All photos associated with this tag will be detached (photos will not been deleted).</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteTag">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showDeleteTagConfirmModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete a tag -->

    <!-- This part is for modal of edit a tag's information -->
    <n-modal v-model:show="showEditTagModal">
      <n-card title="Edit Gallery" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>
                <n-input v-model:value="editTagInfo.tagName" type="text" placeholder="Tag Name" />
                <n-button strong secondary style="width: 100%; " @click="editTag">
                  Submit
                </n-button>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit a tag's information -->
    

    <!-- This part is for modal of confirming to delete multiple photos -->
    <n-modal v-model:show="showPhotoMultiDeleteModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE these photos?
        <p>*All photos will be destroied.</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteSelectedPhoto">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showPhotoMultiDeleteModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete multiple photos -->

    <!-- This part is for modal of confirming to detach multiple photos -->
    <n-modal v-model:show="showPhotoMultiDetachModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Detach"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to Detach these photos from this tag?

        <n-space style="margin-top: 1rem">
          <n-button type="warning" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="removeFromTagSelectedPhoto">
            Confirm
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showPhotoMultiDetachModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to detach multiple photos -->
    
    <!-- ENDOF: MODAL PART -->

    <n-back-top :right="100" />
  </div>

  
  
</template>

<script>

const moveToModuleRef = ref(null);
const moveToMultiModuleRef = ref(null);

export default defineComponent({
  data() {
    return {
      renderPhotoTagIds: [],
      photoChildren: [],
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
      newNameVal: "",
      baseUVideo: "" ,
      showPhotoSpaceMenu: false,
      showTagMenu: false,
      showPhotoSpaceMenuPhotoInfo: null,
      mouseX: null,
      mouseY: null,
      PhotoSpaceMenuOptions: [
        {
          label: "View",
          key: "view",
          icon() {
            return h(NIcon, null, {
              default: () => h(Eye20Regular)
            });
          }
        },
        {
          label: "Download",
          key: "download",
          icon() {
            return h(NIcon, null, {
              default: () => h(ArrowDownload20Filled)
            });
          }
        },
        {
          label: "Delete",
          key: "delete",
          icon() {
            return h(NIcon, null, {
              default: () => h(Delete20Regular)
            });
          }
        },
        {
          label: "Move To",
          key: "move_to",
          icon() {
            return h(NIcon, null, {
              default: () => h(FolderArrowRight20Regular)
            });
          }
        }
      ],
      TagMenuOptions: [
        {
          label: "Edit",
          key: "edit",
          icon() {
            return h(NIcon, null, {
              default: () => h(Edit20Regular)
            });
          }
        },
        {
          label: "Delete",
          key: "delete",
          icon() {
            return h(NIcon, null, {
              default: () => h(Delete20Regular)
            });
          }
        },
      ],
      
      moveToModalOriginalNsId: -1,
      moveToModalDisableOrgNsId: false,
      
      tagChildren: [],
      addNewTagInput: "",

      editTagInfo: null,

      showDeleteTagConfirmModal: false,
      showEditTagModal: false,

      photoMultiSelectMode: false,
      showPhotoMultiDeleteModal: false,
      showPhotoMultiDetachModal: false,
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    showDetach () {
      if (this.renderPhotoTagIds.length === 0)
      {
        window.$message.warning("Cannot operate when no tags is selected!")
      }else if (this.renderPhotoTagIds.length > 1)
      {
        window.$message.warning("Cannot operate when multiple tags are selected!")
      }else {
        this.showPhotoMultiDetachModal = true;
      }
    },
    moveSelectedPhoto() {
      this.moveToModalOriginalNsId = this.nsId;
      this.moveToModalDisableOrgNsId = false;
      this.openMoveToModalMulti();
    },
    openMoveToModalMulti () {
      moveToMultiModuleRef.value.openModal();
    },
    moveItemMulti (nsId) {
      // if it is true then this is a folder move
      if (this.moveToModalDisableOrgNsId)
      {
        
      }else {
        // else this is a photo move
        this.photoChildren.forEach((photo) => {
          if (photo["checked"])
          {
            axios({
              method: 'post',
              baseURL: '',
              url: import.meta.env.VITE_APP_BASE_URL + "/photo/changeNamespace?photoId=" + photo.photoId + "&nsId=" + nsId,
              headers: {
                "HRD-Token": localStorage.getItem("HRD-Token")
              },
              data: {
            }
            }).then((response) => {
              if (response.data.msgCode === 200)
              {
                this.queryPhotos();
                window.$message.success("Media Location Updated!");
                
              }else {
                window.$message.warning("Update Error!")
              }
            })
            .catch(function (error) { // 请求失败处理
              window.$message.error(error);
            });
          }
        })
        this.moveToModalOriginalNsId = -1;
        this.moveToModalDisableOrgNsId = false;

        this.photoMultiSelectMode = false;
        this.clearCheckboxPhoto();
      }
      
    },
    removeFromTagSelectedPhoto () {
      this.photoChildren.forEach((photo) => {
        if(photo["checked"])
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteTag?photoId=" + photo.photoId + "&tagId=" + this.renderPhotoTagIds[0],
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              this.queryPhotos();
              window.$message.success("Tag detached!");
            }else {
              window.$message.error("removeTag Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
        }
      })
      this.showPhotoMultiDetachModal = false;
      this.photoMultiSelectMode = false;
    },
    deleteSelectedPhoto () {
      this.photoChildren.forEach((photo) => {
        if(photo["checked"])
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/photo/delete?photoId=" + photo.photoId,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              this.queryPhotos();
              window.$message.success("Photo deleted!");
              
            }else {
              window.$message.warning("delete photo Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            window.$message.error(error);
          });
        }
      })
      this.showPhotoMultiDeleteModal = false;
      this.clearCheckboxPhoto();
      this.photoMultiSelectMode = false;
    },
    selectAllCheckboxPhoto () {
      this.photoChildren.forEach((photo) => {
        photo["checked"] = true;
      })
    },
    clearCheckboxPhoto () {
      this.photoChildren.forEach((photo) => {
        photo["checked"] = false;
      })
    },
    handlePhotoMulti(photo, event)
    {
      if (this.photoMultiSelectMode)
      {
        event.preventDefault();
        event.stopPropagation();
      }else {
        this.photoModalOpen(photo);
      }
    },
    blankSpaceMenu (e) {
      e.preventDefault();
      e.stopPropagation();
    },
    handleSelectPhoto(key) {
      this.showPhotoSpaceMenu = false;
      if (key === "view")
      {
        this.photoModalOpen(this.showPhotoSpaceMenuPhotoInfo);
      }
      if (key === "download")
      {
        this.showPhotoInfo = this.showPhotoSpaceMenuPhotoInfo;
        this.download();
      }
      if (key === "delete")
      {
        this.deletePhoto();
      }
      if (key === "move_to")
      {
        this.moveToModalOriginalNsId = this.nsId;
        this.moveToModalDisableOrgNsId = false;
        this.openMoveToModal();
      }
    },
    handleSelectTag(key) {
      this.showTagMenu = false;
      if (key === "edit")
      {
        this.showEditTagModal = true;
      }
      if (key === "delete")
      {
        this.showDeleteTagConfirmModal = true;
      }
    },
    onClickOutsidePhoto () {
      this.showPhotoSpaceMenu = false;
    },
    onClickOutsideTag () {
      this.showTagMenu = false;
    },
    photoSpaceMenu(photo, e) {
      this.showPhotoSpaceMenuPhotoInfo = photo;
      e.preventDefault();
      e.stopPropagation();
      this.showPhotoSpaceMenu = false;
      nextTick().then(() => {
        this.showPhotoSpaceMenu = true;
        this.mouseX = e.clientX;
        this.mouseY = e.clientY;
        
        });
    },
    TagMenu(tag, e) {
      this.editTagInfo = {
        "tagId": tag.tagId,
        "tagName": tag.tagName
      };
      e.preventDefault();
      e.stopPropagation();
      this.mouseX = e.clientX;
      this.mouseY = e.clientY;
      this.showTagMenu = true;
    },
    deleteTag () {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/delete?tagId=" + this.editTagInfo.tagId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.getAllTags();
            this.photoChildren = []
            this.showDeleteTagConfirmModal = false;
            window.$message.success("Tag Deleted!");
            this.renderPhotoTagIds = this.renderPhotoTagIds.filter(tagId => tagId !== this.editTagInfo.tagId)
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    moveItem (nsId) {
      // if it is true then this is a folder move
      if (this.moveToModalDisableOrgNsId)
      {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/updateParent",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "nsParentId": nsId,
            "nsId": this.showOperateFolderConfirmNsId
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryNsChildren();
            window.$message.success("Folder Location Updated!");
            this.moveToModalOriginalNsId = -1;
            this.moveToModalDisableOrgNsId = false;
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }else {
        // else this is a photo move

        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/changeNamespace?photoId=" + this.showPhotoSpaceMenuPhotoInfo.photoId + "&nsId=" + nsId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryPhotos();
            window.$message.success("Media Location Updated!");
            this.moveToModalOriginalNsId = -1;
            this.moveToModalDisableOrgNsId = false;
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
      
    },
    openMoveToModal () {
      moveToModuleRef.value.openModal();
    },
    deletePhoto() {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/delete?photoId=" + this.showPhotoSpaceMenuPhotoInfo.photoId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryPhotos();
            window.$message.success("Photo deleted!")
          }else {
            window.$message.warning("User Info Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    isphotoOrVideo (format) {
      format = format.toLowerCase();
      let photoFormats = ["jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "svg", "psd", "raw", "arw", "pcd", "jfif"]
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
    changeName() {
      if (this.newNameVal === "" || this.newNameVal === null)
      {
        window.$message.warning("Image name cannot be empty!")
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/update",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "photoId": this.showPhotoInfo.photoId,
            "photoName": this.newNameVal
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.showPhotoInfo.photoName = this.newNameVal;
            this.queryPhotos();
            window.$message.success("Photo name changed!")
          }else {
            window.$message.warning("Name Change Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
      
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
          window.$message.success("visibility changed!")
        }else {
          window.$message.warning("change visibility Response Error!")
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
          window.$message.warning("share addr Response Error!")
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
          window.$message.error("add Tag Response Error!")
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
          window.$message.success("added to gallery!");
          this.queryPhotos();

        }else {
          window.$message.error("addGa Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    createTag() {
      if (this.addNewTagInput === null || this.addNewTagInput === "")
      {
        window.$message.warning("Please Input a Valid Tag Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/insert",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "tagName": this.addNewTagInput,
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Created a New Tag!");
            this.getAllTags();
            this.addNewTagInput = "";
          }else {
            window.$message.error("createTag Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
    editTag() {
      if (this.editTagInfo.tagName === null || this.editTagInfo.tagName === "")
      {
        window.$message.warning("Please Input a Valid Tag Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/tag/update",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: this.editTagInfo
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Information Updated!");
            this.getAllTags();
            this.showEditTagModal = false;
          }else {
            window.$message.error("Update Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
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
          this.tagChildren = infoT;
        }else {
          window.$message.error("Tag Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
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
          window.$message.error("getGa Response Error!")
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
          this.queryPhotos();
          window.$message.success("Gallery detached!")
        }else {
          window.$message.error("removeGa Response Error!")
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
          window.$message.success("Tag detached!")
        }else {
          window.$message.error("removeTag Response Error!")
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
          window.$message.error("get tags Response Error!")
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
          window.$message.error("get gallery Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    trace(nsId) {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/trace?nsId=" + nsId,
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
          window.$message.error("trace Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    photoModalOpen(photoInfo) {
      this.showPhotoInfo = photoInfo;
      this.showPhotoInfoVisibility = this.showPhotoInfo.visibility === 1 ? true : false;
      this.newNameVal = this.showPhotoInfo.photoName;
      this.baseUVideo = import.meta.env.VITE_APP_BASE_URL + "/photo/renderV/" + this.userToken + "?photoId=" + this.showPhotoInfo.photoId
      this.getTags();
      this.getGas();
      this.trace(photoInfo.nsId)

      this.showPhoto = true;
    },
    queryPhotos() {
      var queryTagStr = "";
      this.renderPhotoTagIds.forEach((tagId) => {queryTagStr += tagId + ","})
      queryTagStr = queryTagStr.substring(0, queryTagStr.length - 1)
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getByTags?tagIds=" + queryTagStr,
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
          window.$message.error("query photo Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.getAllTags();
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
.tagCard {
  cursor: pointer;
  user-select: none;
}
</style>
